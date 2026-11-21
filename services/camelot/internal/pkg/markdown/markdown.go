// Package markdown renders Markdown (CommonMark with GitHub tables, task
// lists, strikethrough and autolinks) to HTML with goldmark's safe
// defaults: raw HTML in the source is omitted and dangerous link schemes
// are dropped.
package markdown

import (
	"bytes"
	"fmt"

	"github.com/yuin/goldmark/v2/extension"
	"github.com/yuin/goldmark/v2/parser"
	"github.com/yuin/goldmark/v2/renderer/html"
)

// HTML is goldmark's safe-mode output: the only string pages may write unescaped.
type HTML string

// Render returns source as HTML.
func Render(source []byte) (HTML, error) {
	p := parser.New(parser.WithExtensions(extension.NewGFMParser()), parser.WithAutoHeadingID())
	r := html.New(html.WithExtensions(extension.NewGFMHTMLRenderer()))
	var buf bytes.Buffer
	if err := r.Render(&buf, source, p.Parse(source)); err != nil {
		return "", fmt.Errorf("markdown: render: %w", err)
	}
	return HTML(buf.String()), nil
}
