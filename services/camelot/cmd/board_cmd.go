package cmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Wigata-Intech/w-tools/cli"

	"github.com/mrdhira/camelot/services/camelot/config"
	"github.com/mrdhira/camelot/services/camelot/internal/pkg/taskboard"
	"github.com/mrdhira/camelot/services/camelot/internal/taskmodel"
)

// importedCard is one line of the import report.
type importedCard struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	Type      string   `json:"type"`
	Status    string   `json:"status"`
	Milestone string   `json:"milestone,omitempty"`
	Folder    string   `json:"folder,omitempty"`
	DoneOn    string   `json:"done_on,omitempty"`
	Waiting   string   `json:"waiting,omitempty"`
	AlsoIDs   []string `json:"also_ids,omitempty"`
	Guessed   []string `json:"guessed,omitempty"`
	Result    string   `json:"result"`
}

// importBoardCommand builds "import-board": a dry run by default, --apply writes.
func importBoardCommand(load func() (*config.Config, error), out, logOut io.Writer) *cli.Command {
	var file, prefix string
	var apply bool
	return &cli.Command{
		Name:  "import-board",
		Short: "import the Markdown board (tasks/TASK.md) as cards; a dry run unless --apply",
		Flags: func(fs *cli.FlagSet) {
			fs.StringVar(&file, "file", "", "the board; default <vault>/tasks/TASK.md")
			fs.StringVar(&prefix, "only-prefix", "", "import only cards whose ID starts with this (and their milestones)")
			fs.BoolVar(&apply, "apply", false, "write the cards (default: report what would be written)")
		},
		Run: runWith(load, logOut, func(ctx context.Context, a *deps) error {
			if file == "" {
				file = filepath.Join(a.config.Vault, "tasks", "TASK.md")
			}
			f, err := os.Open(file) //nolint:gosec // the operator names the board to read
			if err != nil {
				return err
			}
			defer func() { _ = f.Close() }()
			board, err := taskboard.Parse(f)
			if err != nil {
				return err
			}
			existing, err := a.tasks.List(ctx, nil)
			if err != nil {
				return err
			}
			have := map[string]bool{}
			for _, c := range existing {
				have[c.ID] = true
			}
			var report []importedCard
			var failed int
			add := func(request *taskmodel.CreateRequest, r importedCard) {
				switch {
				case have[request.ID]:
					r.Result = "exists, skipped"
				case !apply:
					r.Result = "would create"
				default:
					if _, err := a.tasks.Create(ctx, request); err != nil {
						r.Result = "error: " + err.Error()
						failed++
					} else {
						r.Result = "created"
						have[request.ID] = true
					}
				}
				report = append(report, r)
			}
			used := map[string]bool{}
			for _, c := range board.Cards {
				if prefix == "" || strings.HasPrefix(c.ID, prefix) {
					used[c.Milestone] = true
				}
			}
			for _, m := range board.Milestones {
				if prefix != "" && !used[m.ID] {
					continue
				}
				request := &taskmodel.CreateRequest{ID: m.ID, Title: m.Title, Type: string(taskmodel.TypeMilestone), Status: m.Status,
					Description: "Spec: " + m.Spec + "\n\nBoard status: " + m.Note, Actor: a.config.Actor, Note: "imported from TASK.md", Imported: true, CreatedAt: time.Now().UTC()}
				add(request, importedCard{ID: m.ID, Title: m.Title, Type: request.Type, Status: m.Status})
			}
			for _, c := range board.Cards {
				if prefix != "" && !strings.HasPrefix(c.ID, prefix) {
					continue
				}
				folder := resolveFolder(a.config.Vault, c.ID, c.Folder)
				at := time.Now().UTC()
				if !c.DoneOn.IsZero() {
					at = c.DoneOn
				}
				description := "Imported from TASK.md (" + c.Section + "):\n\n" + c.Description
				if len(c.AlsoIDs) > 0 {
					description += "\n\nAlso covers: " + strings.Join(c.AlsoIDs, ", ")
				}
				note := "imported from TASK.md"
				if c.Status == string(taskmodel.StatusDone) && c.DoneOn.IsZero() {
					note = taskmodel.NoteImportedUndated
				}
				request := &taskmodel.CreateRequest{ID: c.ID, MilestoneID: c.Milestone, Title: c.Title, Type: c.Type, Status: c.Status, Waiting: c.Waiting,
					Folder: folder, Description: description, Actor: a.config.Actor, Note: note, Imported: true, CreatedAt: at}
				r := importedCard{ID: c.ID, Title: c.Title, Type: c.Type, Status: c.Status, Milestone: c.Milestone, Folder: folder, Waiting: c.Waiting, AlsoIDs: c.AlsoIDs, Guessed: c.Guessed}
				if !c.DoneOn.IsZero() {
					r.DoneOn = c.DoneOn.Format(time.DateOnly)
				}
				add(request, r)
			}
			if err := writeJSON(out, map[string]any{"applied": apply, "cards": report, "skipped_lines": board.Skipped}); err != nil {
				return err
			}
			if failed > 0 {
				return fmt.Errorf("%d cards failed", failed)
			}
			return nil
		}),
	}
}

// resolveFolder returns the board's folder when it exists, else the
// single tasks/ or tasks/_archive/ directory named "<id>-…", else "";
// read through an os.Root on the vault, so no link leaves it.
func resolveFolder(vault, id, folder string) string {
	root, err := os.OpenRoot(vault)
	if err != nil {
		return ""
	}
	defer func() { _ = root.Close() }()
	if folder != "" {
		if info, err := root.Stat(folder); err == nil && info.IsDir() {
			return folder
		}
	}
	for _, dir := range []string{"tasks", "tasks/_archive"} {
		matches, _ := fs.Glob(root.FS(), dir+"/"+id+"-*")
		if len(matches) == 1 {
			return matches[0]
		}
	}
	return ""
}

// exportBoardCommand builds "export-board": the cards as a plain-text Markdown board.
func exportBoardCommand(load func() (*config.Config, error), out, logOut io.Writer) *cli.Command {
	var file string
	return &cli.Command{
		Name:  "export-board",
		Short: "write the cards as a read-only Markdown board (stdout, or --out)",
		Flags: func(fs *cli.FlagSet) {
			fs.StringVar(&file, "out", "", "write to this file (atomically) instead of stdout")
		},
		Run: runWith(load, logOut, func(ctx context.Context, a *deps) error {
			cards, err := a.tasks.List(ctx, nil)
			if err != nil {
				return err
			}
			var buf bytes.Buffer
			fmt.Fprintf(&buf, "# TASK.md — generated by Camelot, %s\n\n", time.Now().UTC().Format(time.RFC3339))
			buf.WriteString("Read-only view. Change cards in Camelot (UI, `camelot task …` or MCP); this file is rewritten.\n")
			for _, st := range taskmodel.Statuses {
				fmt.Fprintf(&buf, "\n## %s\n\n", st.Label())
				for _, c := range cards {
					if c.Status != st {
						continue
					}
					parts := []string{string(c.Type)}
					if c.MilestoneID != "" {
						parts = append(parts, c.MilestoneID)
					}
					if c.Assignee != "" {
						parts = append(parts, "@"+c.Assignee)
					}
					if len(c.Deps) > 0 {
						parts = append(parts, "deps "+strings.Join(c.Deps, ", "))
					}
					if c.Waiting != "" {
						parts = append(parts, "waiting on Dhira: "+c.Waiting)
					}
					line := fmt.Sprintf("- [%s] %s — %s", c.ID, c.Title, strings.Join(parts, " · "))
					if c.Folder != "" {
						line += " → `" + strings.TrimPrefix(c.Folder, "tasks/") + "/`"
					}
					buf.WriteString(line + "\n")
				}
			}
			if file == "" {
				_, err := out.Write(buf.Bytes())
				return err
			}
			return writeFileAtomic(file, buf.Bytes())
		}),
	}
}

// writeFileAtomic replaces path through a temporary file and a rename,
// keeping an existing file's mode (0644 for a new one).
func writeFileAtomic(path string, content []byte) error {
	mode := os.FileMode(0o644)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".camelot-*.tmp")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	_, werr := tmp.Write(content)
	merr := tmp.Chmod(mode)
	serr := tmp.Sync()
	cerr := tmp.Close()
	if err := errors.Join(werr, merr, serr, cerr); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
