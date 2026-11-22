package searchservice_test

import (
	"testing"

	"github.com/mrdhira/camelot/services/camelot/internal/searchservice"
)

func TestSnippet(t *testing.T) {
	tests := []struct {
		name     string
		input    [2]string
		expected string
		found    bool
	}{
		{name: "a match in the middle, case-insensitive", input: [2]string{"The payment   links\nare live", "LINKS"}, expected: "…The payment links are live…", found: true},
		{name: "lower-casing changes byte lengths (İ, Kelvin sign)", input: [2]string{"İİİİ ﬁle KKKK target here", "target"}, expected: "…İİİİ ﬁle KKKK target here…", found: true},
		{name: "a match ending in a multi-byte rune", input: [2]string{"naïve café", "CAFÉ"}, expected: "…naïve café…", found: true},
		{name: "no match", input: [2]string{"abc", "xyz"}},
		{name: "an empty query", input: [2]string{"abc", ""}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := searchservice.Snippet(tt.input[0], tt.input[1])
			if ok != tt.found || got != tt.expected {
				t.Errorf("Snippet() = %q, %v; want %q, %v", got, ok, tt.expected, tt.found)
			}
		})
	}
}

func FuzzSnippet(f *testing.F) {
	f.Add("İİ Kelvin K text", "k")
	f.Fuzz(func(t *testing.T, text, q string) {
		_, _ = searchservice.Snippet(text, q)
	})
}
