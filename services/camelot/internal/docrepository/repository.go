// Package docrepository reads and updates the document files.
package docrepository

import (
	"context"

	"github.com/mrdhira/camelot/services/camelot/internal/docmodel"
	"github.com/mrdhira/camelot/services/camelot/internal/docrepository/markdown"
)

// Repository is the document store.
type Repository interface {
	// DocSelect returns every document under the roots and the problems
	// found: a Markdown file whose frontmatter has a type is a document.
	DocSelect(ctx context.Context) ([]*docmodel.Doc, []docmodel.Problem, error)
	// DocUpdateFields rewrites the given frontmatter keys of the document
	// at path (relative to the vault), keeping every other byte.
	DocUpdateFields(ctx context.Context, path string, fields map[string]string) error
}

// NewMarkdown returns the Repository on the Markdown files under roots
// (relative to vault).
func NewMarkdown(vault string, roots []string) Repository {
	return markdown.New(vault, roots)
}
