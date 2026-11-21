// Package markdown is the document store on Markdown files with
// frontmatter.
package markdown

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mrdhira/camelot/services/camelot/internal/docmodel"
	"github.com/mrdhira/camelot/services/camelot/internal/pkg/frontmatter"
)

// maxFileSize bounds a document file.
const maxFileSize = 4 << 20

// skipDirs are directories never walked.
var skipDirs = []string{".git", "node_modules", "_archive", "references"}

// Store reads the documents under roots, relative to vault.
type Store struct {
	vault string
	roots []string
}

// New returns the store.
func New(vault string, roots []string) *Store {
	return &Store{vault: vault, roots: roots}
}

// DocSelect walks the roots.
func (s *Store) DocSelect(ctx context.Context) ([]*docmodel.Doc, []docmodel.Problem, error) {
	var docs []*docmodel.Doc
	var problems []docmodel.Problem
	for _, root := range s.roots {
		dir := filepath.Join(s.vault, root)
		if _, err := os.Stat(dir); errors.Is(err, fs.ErrNotExist) {
			problems = append(problems, docmodel.Problem{Path: root, Message: "document root does not exist"})
			continue
		}
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if d.IsDir() {
				if slices.Contains(skipDirs, d.Name()) {
					return filepath.SkipDir
				}
				return nil
			}
			if filepath.Ext(path) != ".md" || !d.Type().IsRegular() {
				return nil
			}
			rel, err := filepath.Rel(s.vault, path)
			if err != nil {
				return err
			}
			doc, problem := s.read(filepath.ToSlash(rel))
			if problem != nil {
				problems = append(problems, *problem)
			}
			if doc != nil {
				docs = append(docs, doc)
			}
			return nil
		})
		if err != nil {
			return nil, nil, fmt.Errorf("doc select %s: %w", root, err)
		}
	}
	return docs, problems, nil
}

// read parses one file, read through the vault root so no link leaves
// it; a file without frontmatter or without a type is not a document.
func (s *Store) read(rel string) (*docmodel.Doc, *docmodel.Problem) {
	root, err := os.OpenRoot(s.vault)
	if err != nil {
		return nil, &docmodel.Problem{Path: rel, Message: err.Error()}
	}
	defer func() { _ = root.Close() }()
	info, err := root.Stat(rel)
	if err != nil {
		return nil, &docmodel.Problem{Path: rel, Message: err.Error()}
	}
	if info.Size() > maxFileSize {
		return nil, &docmodel.Problem{Path: rel, Message: "file larger than 4 MiB, skipped"}
	}
	content, err := root.ReadFile(rel)
	if err != nil {
		return nil, &docmodel.Problem{Path: rel, Message: err.Error()}
	}
	fm, err := frontmatter.Parse(content)
	if errors.Is(err, frontmatter.ErrNone) {
		return nil, nil
	}
	if err != nil {
		return nil, &docmodel.Problem{Path: rel, Message: err.Error()}
	}
	typeField, ok := fm.Get(docmodel.KeyType)
	if !ok {
		return nil, nil
	}
	doc := &docmodel.Doc{Path: rel, Body: fm.Body, Serves: []string{}}
	var msgs []string
	if doc.Type, err = docmodel.ParseType(typeField.Value); err != nil {
		return nil, &docmodel.Problem{Path: rel, Message: err.Error()}
	}
	get := func(key string) string {
		f, _ := fm.Get(key)
		return f.Value
	}
	if doc.ID, err = docmodel.ParseID(get(docmodel.KeyID)); err != nil {
		return nil, &docmodel.Problem{Path: rel, Message: err.Error()}
	}
	doc.Title = get(docmodel.KeyTitle)
	if doc.Title == "" {
		msgs = append(msgs, "title is missing")
		doc.Title = doc.ID
	}
	if doc.Status, err = docmodel.ParseStatus(get(docmodel.KeyStatus)); err != nil {
		msgs = append(msgs, err.Error())
		doc.Status = docmodel.StatusDraft
	}
	doc.Owner = get(docmodel.KeyOwner)
	doc.Updated = get(docmodel.KeyUpdated)
	if f, ok := fm.Get(docmodel.KeyServes); ok {
		items := f.List
		if !f.IsList && f.Value != "" {
			items = []string{f.Value}
		}
		for _, item := range items {
			id, err := docmodel.ParseID(item)
			if err != nil {
				msgs = append(msgs, "serves: "+err.Error())
				continue
			}
			doc.Serves = append(doc.Serves, id)
		}
	}
	if len(msgs) > 0 {
		return doc, &docmodel.Problem{Path: rel, DocID: doc.ID, Message: strings.Join(msgs, "; ")}
	}
	return doc, nil
}

// DocUpdateFields rewrites the given keys of the file at rel, through
// the vault root: a temporary file beside it, then a rename.
func (s *Store) DocUpdateFields(ctx context.Context, rel string, fields map[string]string) error {
	if err := s.checkRoots(rel); err != nil {
		return err
	}
	root, err := os.OpenRoot(s.vault)
	if err != nil {
		return fmt.Errorf("doc update: %w", err)
	}
	defer func() { _ = root.Close() }()
	info, err := root.Stat(rel)
	if err != nil {
		return fmt.Errorf("doc update: %w", err)
	}
	content, err := root.ReadFile(rel)
	if err != nil {
		return fmt.Errorf("doc update: %w", err)
	}
	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		if content, err = frontmatter.Set(content, k, fields[k]); err != nil {
			return fmt.Errorf("doc update %s: %w", rel, err)
		}
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return replace(root, rel, content, info.Mode().Perm())
}

// replace writes content to a new, uniquely named temporary file beside
// rel (exclusive create, so concurrent writers never share one), syncs it
// and renames it over rel.
func replace(root *os.Root, rel string, content []byte, perm os.FileMode) error {
	tmp := path.Join(path.Dir(rel), "."+path.Base(rel)+"."+rand.Text()+".tmp")
	f, err := root.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return fmt.Errorf("doc write: %w", err)
	}
	_, werr := f.Write(content)
	serr := f.Sync()
	cerr := f.Close()
	if err := errors.Join(werr, serr, cerr); err != nil {
		_ = root.Remove(tmp)
		return fmt.Errorf("doc write: %w", err)
	}
	if err := root.Rename(tmp, rel); err != nil {
		_ = root.Remove(tmp)
		return fmt.Errorf("doc write: %w", err)
	}
	return nil
}

// checkRoots refuses a path outside the document roots or not Markdown.
func (s *Store) checkRoots(rel string) error {
	clean := path.Clean(rel)
	if path.Ext(clean) == ".md" && !strings.HasPrefix(clean, "../") {
		for _, root := range s.roots {
			if strings.HasPrefix(clean, path.Clean(filepath.ToSlash(root))+"/") {
				return nil
			}
		}
	}
	return fmt.Errorf("%w: path %q is outside the document roots", docmodel.ErrInvalid, rel)
}
