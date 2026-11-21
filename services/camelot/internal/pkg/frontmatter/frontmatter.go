// Package frontmatter reads and rewrites a strict flat subset of YAML
// frontmatter: a first line "---", then lines of `key: value`, blank lines
// or whole-line `#` comments, closed by "---". A value is plain text, a
// double-quoted Go string, or a list `[a, b]` of plain items. Anything else
// is an error, so a typo never silently drops a field.
package frontmatter

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// delimiter opens and closes the block.
const delimiter = "---"

// ErrNone means the content does not start with a frontmatter block.
var ErrNone = errors.New("frontmatter: no frontmatter block")

// keyPattern is a field name: lower-case letters, digits and underscores.
var keyPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// Field is one `key: value` line; List is set when the value was a list.
type Field struct {
	Key    string
	Value  string
	List   []string
	IsList bool
}

// Document is a parsed block: its fields in file order and the body after it.
type Document struct {
	Fields []Field
	Body   []byte
}

// Get returns the value of key and whether it is present.
func (d *Document) Get(key string) (Field, bool) {
	for _, f := range d.Fields {
		if f.Key == key {
			return f, true
		}
	}
	return Field{}, false
}

// Parse splits content into its frontmatter fields and body. Content
// without a leading "---" line returns ErrNone.
func Parse(content []byte) (*Document, error) {
	lines, bodyStart, err := block(content)
	if err != nil {
		return nil, err
	}
	doc := &Document{Body: content[bodyStart:]}
	seen := map[string]bool{}
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		f, err := parseLine(line)
		if err != nil {
			return nil, fmt.Errorf("frontmatter: line %d: %w", i+2, err)
		}
		if seen[f.Key] {
			return nil, fmt.Errorf("frontmatter: line %d: duplicate key %q", i+2, f.Key)
		}
		seen[f.Key] = true
		doc.Fields = append(doc.Fields, f)
	}
	return doc, nil
}

// Set returns content with key's value replaced by value (a new line
// appended before the closing "---" when key is absent), every other byte
// kept, line endings included. value is quoted when the plain form would not read back unchanged.
func Set(content []byte, key, value string) ([]byte, error) {
	if !keyPattern.MatchString(key) {
		return nil, fmt.Errorf("frontmatter: invalid key %q", key)
	}
	lines, bodyStart, err := block(content)
	if err != nil {
		return nil, err
	}
	newLine := key + ": " + Quote(value)
	replaced := false
	for i, line := range lines {
		if k, _, ok := strings.Cut(line, ":"); ok && strings.TrimSpace(k) == key && !strings.HasPrefix(strings.TrimSpace(line), "#") {
			lines[i] = newLine
			replaced = true
			break
		}
	}
	if !replaced {
		lines = append(lines, newLine)
	}
	eol := "\n"
	if strings.HasPrefix(string(content), delimiter+"\r\n") {
		eol = "\r\n"
	}
	var out bytes.Buffer
	out.WriteString(delimiter + eol)
	for _, line := range lines {
		out.WriteString(line)
		out.WriteString(eol)
	}
	out.WriteString(delimiter + eol)
	out.Write(content[bodyStart:])
	return out.Bytes(), nil
}

// Quote returns value as written in a block: plain when it reads back
// unchanged, else a double-quoted Go string.
func Quote(value string) string {
	plain := value != "" &&
		value == strings.TrimSpace(value) &&
		!strings.Contains(value, ": ") &&
		!strings.HasSuffix(value, ":") &&
		!strings.ContainsAny(value[:1], `"'[]{}#&*!|>%@`+"`") &&
		!strings.ContainsAny(value, "\n\r\t")
	if plain {
		return value
	}
	return strconv.Quote(value)
}

// block returns the lines between the delimiters and the offset of the
// body.
func block(content []byte) ([]string, int, error) {
	text := string(content)
	offset := 0
	var lines []string
	for i := 0; ; i++ {
		end := strings.IndexByte(text[offset:], '\n')
		next := offset + end + 1
		if end < 0 {
			next = len(text)
			end = len(text) - offset
		}
		line := strings.TrimRight(text[offset:offset+end], "\r")
		switch {
		case i == 0 && line != delimiter:
			return nil, 0, ErrNone
		case i > 0 && line == delimiter:
			return lines, next, nil
		case i > 0:
			lines = append(lines, line)
		}
		if next >= len(text) {
			return nil, 0, errors.New("frontmatter: block not closed by ---")
		}
		offset = next
	}
}

// parseLine reads one `key: value` line.
func parseLine(line string) (Field, error) {
	key, raw, ok := strings.Cut(line, ":")
	if !ok {
		return Field{}, fmt.Errorf("not a key: value line: %q", line)
	}
	if !keyPattern.MatchString(key) {
		return Field{}, fmt.Errorf("invalid key %q (lower-case letters, digits, _; no indentation)", key)
	}
	raw = strings.TrimSpace(raw)
	f := Field{Key: key}
	switch {
	case strings.HasPrefix(raw, "["):
		if !strings.HasSuffix(raw, "]") {
			return Field{}, fmt.Errorf("%s: list not closed by ]", key)
		}
		f.IsList = true
		inner := strings.TrimSpace(raw[1 : len(raw)-1])
		if inner != "" {
			for item := range strings.SplitSeq(inner, ",") {
				item = strings.TrimSpace(item)
				if item == "" {
					return Field{}, fmt.Errorf("%s: empty list item", key)
				}
				f.List = append(f.List, item)
			}
		}
		f.Value = strings.Join(f.List, ", ")
	case strings.HasPrefix(raw, `"`):
		v, err := strconv.Unquote(raw)
		if err != nil {
			return Field{}, fmt.Errorf("%s: bad quoted value: %w", key, err)
		}
		f.Value = v
	default:
		f.Value = raw
	}
	return f, nil
}
