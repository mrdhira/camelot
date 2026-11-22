package markdown_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/mrdhira/camelot/services/camelot/internal/pkg/markdown"
)

func TestRender(t *testing.T) {
	type expected struct {
		contains []string
		absent   []string
	}
	tests := []struct {
		name     string
		input    string
		expected expected
	}{
		{
			name:     "headings, emphasis and a GFM table",
			input:    "# Title\n\n**bold**\n\n| a | b |\n|---|---|\n| 1 | 2 |\n",
			expected: expected{contains: []string{`<h1 id="title">Title</h1>`, "<strong>bold</strong>", "<table>", "<td>1</td>"}},
		},
		{
			name:     "raw HTML is omitted",
			input:    "<script>alert(1)</script>\n\ntext <b onclick=x>b</b>\n",
			expected: expected{absent: []string{"<script>", "onclick"}},
		},
		{
			name:     "a javascript: link loses its target",
			input:    "[x](javascript:alert(1))\n",
			expected: expected{absent: []string{"javascript:"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := markdown.Render([]byte(tt.input))
			if err != nil {
				t.Fatalf("Render() error = %v", err)
			}
			for _, s := range tt.expected.contains {
				if !strings.Contains(string(got), s) {
					t.Errorf("Render() = %q, want it to contain %q", got, s)
				}
			}
			for _, s := range tt.expected.absent {
				if strings.Contains(string(got), s) {
					t.Errorf("Render() = %q, want no %q", got, s)
				}
			}
		})
	}
}

func TestHeadings(t *testing.T) {
	html, err := markdown.Render([]byte("# Title\n\n## One & two\n\ntext\n\n### Sub `code`\n\n#### Deep\n\n## Three\n"))
	if err != nil {
		t.Fatal(err)
	}
	got := markdown.Headings(html)
	want := []markdown.Heading{{Level: 2, ID: "one--two", Text: "One & two"}, {Level: 3, ID: "sub-code", Text: "Sub code"}, {Level: 2, ID: "three", Text: "Three"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Headings() = %+v, want %+v", got, want)
	}
}
