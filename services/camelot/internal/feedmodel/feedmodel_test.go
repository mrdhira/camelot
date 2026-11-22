package feedmodel_test

import (
	"testing"

	"github.com/mrdhira/camelot/services/camelot/internal/feedmodel"
)

func TestParseStamp(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected feedmodel.Stamp
		err      bool
	}{
		{name: "three IDs", input: "c12-d3-m4", expected: feedmodel.Stamp{CardEventID: 12, DocEventID: 3, CommentID: 4}},
		{name: "the two-ID form of an older page", input: "c12-d3", err: true},
		{name: "trailing text", input: "c1-d2-m3x", err: true},
		{name: "a negative ID", input: "c-1-d2-m3", err: true},
		{name: "a leading zero", input: "c01-d2-m3", err: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := feedmodel.ParseStamp(tt.input)
			if (err != nil) != tt.err || got != tt.expected {
				t.Errorf("ParseStamp(%q) = %+v, %v", tt.input, got, err)
			}
		})
	}
}
