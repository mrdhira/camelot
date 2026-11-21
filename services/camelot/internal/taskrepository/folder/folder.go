// Package folder reads the Markdown files of card folders, through an
// os.Root on the vault so no path or link leaves it.
package folder

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"strings"

	"github.com/mrdhira/camelot/services/camelot/internal/taskmodel"
)

// maxFileSize bounds a file read.
const maxFileSize = 2 << 20

// Store reads card folders under vault.
type Store struct {
	vault string
}

// New returns the store.
func New(vault string) *Store {
	return &Store{vault: vault}
}

// FolderFiles lists the Markdown files directly in folder.
func (s *Store) FolderFiles(ctx context.Context, folder string) ([]string, error) {
	root, err := os.OpenRoot(s.vault)
	if err != nil {
		return nil, fmt.Errorf("folder files: %w", err)
	}
	defer func() { _ = root.Close() }()
	entries, err := fs.ReadDir(root.FS(), folder)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("%w: %s", taskmodel.ErrFolderMissing, folder)
	}
	if err != nil {
		return nil, fmt.Errorf("folder files: %w", err)
	}
	files := []string{}
	for _, e := range entries {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if e.Type().IsRegular() && strings.HasSuffix(e.Name(), ".md") {
			files = append(files, e.Name())
		}
	}
	return files, nil
}

// FolderFileRead returns one Markdown file of folder, at most 2 MiB.
func (s *Store) FolderFileRead(ctx context.Context, folder, name string) ([]byte, error) {
	if name != path.Base(name) || !strings.HasSuffix(name, ".md") || strings.HasPrefix(name, ".") {
		return nil, fmt.Errorf("%w: %s", taskmodel.ErrFileNotFound, name)
	}
	root, err := os.OpenRoot(s.vault)
	if err != nil {
		return nil, fmt.Errorf("folder file: %w", err)
	}
	defer func() { _ = root.Close() }()
	rel := path.Join(folder, name)
	info, err := root.Stat(rel)
	if errors.Is(err, fs.ErrNotExist) || (err == nil && !info.Mode().IsRegular()) {
		return nil, fmt.Errorf("%w: %s", taskmodel.ErrFileNotFound, name)
	}
	if err != nil {
		return nil, fmt.Errorf("folder file: %w", err)
	}
	if info.Size() > maxFileSize {
		return nil, fmt.Errorf("%w: %s is larger than 2 MiB", taskmodel.ErrFileNotFound, name)
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return root.ReadFile(rel)
}
