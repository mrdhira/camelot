// Package markdown renders Markdown (CommonMark with GitHub tables, task
// lists, strikethrough and autolinks) to HTML with goldmark's safe
// defaults: raw HTML in the source is omitted and dangerous link schemes
// are dropped.
package markdown

import (
	"bytes"
	"fmt"
	stdhtml "html"
	"regexp"
	"strings"

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

// Heading is a section heading of rendered Markdown, for a table of contents.
type Heading struct {
	Level int
	ID    string
	Text  string
}

var (
	headingTag = regexp.MustCompile(`(?s)<h([23]) id="([^"]+)">(.*?)</h[23]>`)
	anyTag     = regexp.MustCompile(`<[^>]*>`)
)

// Headings returns the level-2 and level-3 headings of html, in order.
func Headings(html HTML) []Heading {
	var out []Heading
	for _, m := range headingTag.FindAllStringSubmatch(string(html), -1) {
		level := 2
		if m[1] == "3" {
			level = 3
		}
		text := strings.TrimSpace(stdhtml.UnescapeString(anyTag.ReplaceAllString(m[3], "")))
		out = append(out, Heading{Level: level, ID: stdhtml.UnescapeString(m[2]), Text: text})
	}
	return out
}
