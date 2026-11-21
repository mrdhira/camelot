package folder_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mrdhira/camelot/services/camelot/internal/taskmodel"
	"github.com/mrdhira/camelot/services/camelot/internal/taskrepository/folder"
)

// vault holds tasks/A with PLAN.md and notes.txt, a link tasks/A/out.md and
// a folder link tasks/B, both pointing outside the vault.
func vault(t *testing.T) string {
	t.Helper()
	v, outside := t.TempDir(), t.TempDir()
	if err := os.MkdirAll(filepath.Join(v, "tasks", "A"), 0o700); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{"tasks/A/PLAN.md": "# Plan", "tasks/A/notes.txt": "x"} {
		if err := os.WriteFile(filepath.Join(v, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(outside, "secret.md"), []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "secret.md"), filepath.Join(v, "tasks", "A", "out.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(v, "tasks", "B")); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestFolderFiles(t *testing.T) {
	s := folder.New(vault(t))
	tests := []struct {
		name     string
		input    string
		expected []string
		err      error
	}{
		{name: "Markdown files only, links excluded", input: "tasks/A", expected: []string{"PLAN.md"}},
		{name: "a folder link out of the vault", input: "tasks/B", err: errors.New("any")},
		{name: "a missing folder", input: "tasks/C", err: taskmodel.ErrFolderMissing},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := s.FolderFiles(context.Background(), tt.input)
			if tt.err != nil {
				if err == nil || (errors.Is(tt.err, taskmodel.ErrFolderMissing) && !errors.Is(err, taskmodel.ErrFolderMissing)) {
					t.Fatalf("FolderFiles() = %v, %v; want error %v", got, err, tt.err)
				}
				return
			}
			if err != nil || !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("FolderFiles() = %v, %v; want %v", got, err, tt.expected)
			}
		})
	}
}

func TestFolderFileRead(t *testing.T) {
	s := folder.New(vault(t))
	tests := []struct {
		name     string
		input    [2]string
		expected string
		err      bool
	}{
		{name: "a file of the folder", input: [2]string{"tasks/A", "PLAN.md"}, expected: "# Plan"},
		{name: "a link out of the vault", input: [2]string{"tasks/A", "out.md"}, err: true},
		{name: "a path, not a name", input: [2]string{"tasks/A", "../A/PLAN.md"}, err: true},
		{name: "not Markdown", input: [2]string{"tasks/A", "notes.txt"}, err: true},
		{name: "through a folder link", input: [2]string{"tasks/B", "secret.md"}, err: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := s.FolderFileRead(context.Background(), tt.input[0], tt.input[1])
			if tt.err {
				if err == nil {
					t.Fatalf("FolderFileRead() = %q, want an error", got)
				}
				return
			}
			if err != nil || string(got) != tt.expected {
				t.Errorf("FolderFileRead() = %q, %v; want %q", got, err, tt.expected)
			}
		})
	}
}
