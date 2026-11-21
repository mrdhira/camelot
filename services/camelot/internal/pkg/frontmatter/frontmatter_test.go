package frontmatter_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/mrdhira/camelot/services/camelot/internal/pkg/frontmatter"
)

func TestParse(t *testing.T) {
	type expected struct {
		fields []frontmatter.Field
		body   string
		err    bool
		errIs  error
	}
	tests := []struct {
		name     string
		input    string
		expected expected
	}{
		{
			name:  "plain, quoted and list values, comments and blank lines skipped",
			input: "---\nid: PRD002\n# a comment\n\ntitle: \"Payment links: create, pay\"\nserves: [BRD001, OKR-2026Q4]\n---\n# Body\n",
			expected: expected{
				fields: []frontmatter.Field{
					{Key: "id", Value: "PRD002"},
					{Key: "title", Value: "Payment links: create, pay"},
					{Key: "serves", Value: "BRD001, OKR-2026Q4", List: []string{"BRD001", "OKR-2026Q4"}, IsList: true},
				},
				body: "# Body\n",
			},
		},
		{name: "an empty list", input: "---\nserves: []\n---\n", expected: expected{fields: []frontmatter.Field{{Key: "serves", IsList: true}}, body: ""}},
		{name: "CRLF line endings", input: "---\r\nid: A\r\n---\r\nbody", expected: expected{fields: []frontmatter.Field{{Key: "id", Value: "A"}}, body: "body"}},
		{name: "no block", input: "# Title\n", expected: expected{err: true, errIs: frontmatter.ErrNone}},
		{name: "block not closed", input: "---\nid: A\n", expected: expected{err: true}},
		{name: "indented key", input: "---\n  id: A\n---\n", expected: expected{err: true}},
		{name: "not key: value", input: "---\njust text\n---\n", expected: expected{err: true}},
		{name: "duplicate key", input: "---\nid: A\nid: B\n---\n", expected: expected{err: true}},
		{name: "list not closed", input: "---\nserves: [A, B\n---\n", expected: expected{err: true}},
		{name: "empty list item", input: "---\nserves: [A, , B]\n---\n", expected: expected{err: true}},
		{name: "bad quoted value", input: "---\ntitle: \"open\n---\n", expected: expected{err: true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := frontmatter.Parse([]byte(tt.input))
			if tt.expected.err {
				if err == nil {
					t.Fatalf("Parse() error = nil, want an error")
				}
				if tt.expected.errIs != nil && !errors.Is(err, tt.expected.errIs) {
					t.Fatalf("Parse() error = %v, want %v", err, tt.expected.errIs)
				}
				return
			}
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			if !reflect.DeepEqual(doc.Fields, tt.expected.fields) {
				t.Errorf("Fields = %#v, want %#v", doc.Fields, tt.expected.fields)
			}
			if string(doc.Body) != tt.expected.body {
				t.Errorf("Body = %q, want %q", doc.Body, tt.expected.body)
			}
		})
	}
}

func TestSet(t *testing.T) {
	type input struct {
		content    string
		key, value string
	}
	tests := []struct {
		name     string
		input    input
		expected string
	}{
		{
			name:     "replaces a value, keeps every other line and the body",
			input:    input{content: "---\nid: A\n# note\nstatus: draft\n---\nbody\n", key: "status", value: "approved"},
			expected: "---\nid: A\n# note\nstatus: approved\n---\nbody\n",
		},
		{
			name:     "appends a missing key",
			input:    input{content: "---\nid: A\n---\nbody", key: "updated", value: "2026-11-22"},
			expected: "---\nid: A\nupdated: 2026-11-22\n---\nbody",
		},
		{
			name:     "keeps CRLF line endings",
			input:    input{content: "---\r\nid: A\r\nstatus: draft\r\n---\r\nbody\r\n", key: "status", value: "approved"},
			expected: "---\r\nid: A\r\nstatus: approved\r\n---\r\nbody\r\n",
		},
		{
			name:     "quotes a value that would not read back plain",
			input:    input{content: "---\ntitle: x\n---\n", key: "title", value: "A: B"},
			expected: "---\ntitle: \"A: B\"\n---\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := frontmatter.Set([]byte(tt.input.content), tt.input.key, tt.input.value)
			if err != nil {
				t.Fatalf("Set() error = %v", err)
			}
			if string(got) != tt.expected {
				t.Errorf("Set() = %q, want %q", got, tt.expected)
			}
			doc, err := frontmatter.Parse(got)
			if err != nil {
				t.Fatalf("Parse(Set()) error = %v", err)
			}
			if f, _ := doc.Get(tt.input.key); f.Value != tt.input.value {
				t.Errorf("round trip %s = %q, want %q", tt.input.key, f.Value, tt.input.value)
			}
		})
	}
}

func FuzzParse(f *testing.F) {
	f.Add("---\nid: A\ntitle: \"x: y\"\nserves: [A, B]\n---\nbody")
	f.Fuzz(func(t *testing.T, content string) {
		doc, err := frontmatter.Parse([]byte(content))
		if err != nil {
			return
		}
		for _, field := range doc.Fields {
			if field.IsList {
				continue
			}
			out, err := frontmatter.Set([]byte(content), field.Key, field.Value)
			if err != nil {
				t.Fatalf("Set() of a parsed field: %v", err)
			}
			again, err := frontmatter.Parse(out)
			if err != nil {
				t.Fatalf("Parse(Set()) error = %v on %q", err, out)
			}
			if got, _ := again.Get(field.Key); got.Value != field.Value {
				t.Fatalf("round trip %q = %q, want %q", field.Key, got.Value, field.Value)
			}
		}
	})
}
