package cmd_test

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrdhira/camelot/services/camelot/cmd"
)

// vault copies testdata/vault into a temporary directory.
func vault(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	src, err := filepath.Abs("../../../testdata/vault")
	if err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("cp", "-R", src+"/.", dir).CombinedOutput(); err != nil {
		t.Fatalf("copy fixture vault: %v: %s", err, out)
	}
	return dir
}

// run executes the command tree with args on v and returns its exit code
// and everything it wrote: output, logs and the CLI's own stderr.
func run(t *testing.T, v string, args ...string) (int, string) {
	t.Helper()
	var out, logs bytes.Buffer
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stderr := os.Stderr
	os.Stderr = w
	os.Args = append([]string{"camelot", "--vault", v, "--log-level", "error"}, args...)
	code := cmd.Root(&out, &logs).Execute(t.Context())
	os.Stderr = stderr
	_ = w.Close()
	errText, _ := io.ReadAll(r)
	return code, out.String() + logs.String() + string(errText)
}

func TestCLI(t *testing.T) {
	v := vault(t)
	type expected struct {
		code     int
		contains []string
	}
	steps := []struct {
		name     string
		input    []string
		expected expected
	}{
		{name: "create with a prefix and the agent's name", input: []string{"task", "create", "--prefix", "FIX", "--title", "From the CLI", "--type", "bug", "--docs", "PRD900", "--actor", "go-engineer"}, expected: expected{contains: []string{`"id": "FIX-001"`, `"type": "bug"`}}},
		{name: "move", input: []string{"task", "move", "--id", "FIX-001", "--status", "wip", "--note", "build started", "--actor", "go-engineer"}, expected: expected{contains: []string{`"status": "wip"`}}},
		{name: "move needs a status", input: []string{"task", "move", "--id", "FIX-001"}, expected: expected{code: 2, contains: []string{"status"}}},
		{name: "an unknown status names the allowed ones", input: []string{"task", "move", "--id", "FIX-001", "--status", "doing"}, expected: expected{code: 1, contains: []string{"allowed: backlog, todo, plan, wip, qa, review, done"}}},
		{name: "a positional argument is refused", input: []string{"task", "show", "--id", "FIX-001", "extra"}, expected: expected{code: 1, contains: []string{"every value is a flag"}}},
		{name: "show has the history with the actor and note", input: []string{"task", "show", "--id", "FIX-001"}, expected: expected{contains: []string{`"actor": "go-engineer"`, `"note": "build started"`}}},
		{name: "comment on a card as the agent", input: []string{"comment", "add", "--kind", "card", "--id", "FIX-001", "--body", "Built; see the report.", "--actor", "go-engineer"}, expected: expected{contains: []string{`"id": 1`, `"actor": "go-engineer"`}}},
		{name: "comment on a section that is not a heading", input: []string{"comment", "add", "--kind", "doc", "--id", "TRD900", "--section", "nope", "--body", "x"}, expected: expected{code: 1, contains: []string{"is not a heading of TRD900"}}},
		{name: "resolve a comment", input: []string{"comment", "resolve", "--comment", "1", "--note", "answered"}, expected: expected{contains: []string{`"resolved_note": "answered"`}}},
		{name: "list a card's comments", input: []string{"comment", "list", "--kind", "card", "--id", "FIX-001"}, expected: expected{contains: []string{`"open": 0`, `"body": "Built; see the report."`}}},
		{name: "list filters", input: []string{"task", "list", "--status", "wip", "--doc", "PRD900"}, expected: expected{contains: []string{`"id": "FIX-001"`}}},
		{name: "delete needs --yes", input: []string{"task", "delete", "--id", "FIX-001"}, expected: expected{code: 1, contains: []string{"--yes"}}},
		{name: "documents with their problems", input: []string{"doc", "list"}, expected: expected{contains: []string{`"id": "TRD900"`, `"problems": []`}}},
		{name: "document status", input: []string{"doc", "status", "--id", "TRD900", "--status", "in-review"}, expected: expected{contains: []string{`"status": "in-review"`}}},
		{name: "insights with formulas", input: []string{"insights"}, expected: expected{contains: []string{`"formula": "WIP = cards started`}}},
		{name: "check is clean", input: []string{"check"}, expected: expected{contains: []string{`"problems": []`}}},
		{name: "import-board is a dry run by default", input: []string{"import-board"}, expected: expected{contains: []string{`"applied": false`, `"result": "would create"`, `"done_on": "2026-11-02"`}}},
		{name: "import-board --apply", input: []string{"import-board", "--apply"}, expected: expected{contains: []string{`"result": "created"`}}},
		{name: "export-board", input: []string{"export-board"}, expected: expected{contains: []string{"## Work in Progress", "- [FIX-001] From the CLI — bug", "- [FIX-002] Imported done card"}}},
		{name: "backup", input: []string{"backup"}, expected: expected{contains: []string{`"backup":`}}},
	}
	for _, st := range steps {
		code, out := run(t, v, st.input...)
		if code != st.expected.code {
			t.Fatalf("%s: exit %d, want %d; output:\n%s", st.name, code, st.expected.code, out)
		}
		for _, want := range st.expected.contains {
			if !strings.Contains(out, want) {
				t.Errorf("%s: output lacks %q:\n%s", st.name, want, out)
			}
		}
	}
}

func TestCLIRefusesPerCardEnvironment(t *testing.T) {
	v := vault(t)
	t.Setenv("CAMELOT_STATUS", "done")
	code, out := run(t, v, "task", "list")
	if code != 1 || !strings.Contains(out, "CAMELOT_STATUS is set") {
		t.Fatalf("exit %d, output %q; want a refusal naming CAMELOT_STATUS", code, out)
	}
}

func TestCLIOutputIsJSON(t *testing.T) {
	v := vault(t)
	code, out := run(t, v, "task", "list")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, out)
	}
	var cards []map[string]any
	if err := json.Unmarshal([]byte(out), &cards); err != nil {
		t.Fatalf("task list output is not a JSON array: %v\n%s", err, out)
	}
}

func TestCLIRefusesADatabaseServedByAnotherHost(t *testing.T) {
	v := vault(t)
	if code, out := run(t, v, "task", "list"); code != 0 {
		t.Fatalf("first run: exit %d: %s", code, out)
	}
	lock := filepath.Join(v, ".camelot", "server.lock")
	if err := os.WriteFile(lock, []byte(`{"host":"camelot-container","since":"2026-11-22T00:00:00Z"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	code, out := run(t, v, "task", "list")
	if code != 1 || !strings.Contains(out, "served by the Camelot container") {
		t.Fatalf("exit %d, output %q; want a refusal pointing at the container", code, out)
	}
	host, err := os.Hostname()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lock, []byte(`{"host":"`+host+`","since":"2026-11-22T00:00:00Z"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if code, out := run(t, v, "task", "list"); code != 0 {
		t.Fatalf("same host: exit %d: %s", code, out)
	}
}
