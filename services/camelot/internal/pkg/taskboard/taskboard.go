// Package taskboard reads the legacy Markdown board (tasks/TASK.md): the
// milestone table and one bullet per card, "- [CODE] title — deps —
// status", under status sections. Types and milestones are guesses from
// the section and the text; the caller shows them for correction.
package taskboard

import (
	"bufio"
	"io"
	"regexp"
	"strings"
	"time"
)

// Card is one card read from the board.
type Card struct {
	ID          string
	AlsoIDs     []string
	Milestone   string
	Title       string
	Type        string
	Status      string
	Waiting     string
	Folder      string
	DoneOn      time.Time
	Section     string
	Description string
	Guessed     []string
}

// Milestone is one row of the milestone table.
type Milestone struct {
	ID     string
	Title  string
	Spec   string
	Status string
	Note   string
}

// Board is everything read.
type Board struct {
	Milestones []Milestone
	Cards      []Card
	Skipped    []string
}

var (
	cardLine     = regexp.MustCompile(`^- \[([^\]]+)\]\s*(.*)$`)
	idPart       = regexp.MustCompile(`^[A-Z][A-Z0-9]*-[A-Za-z0-9]+`)
	milestoneRow = regexp.MustCompile(`^\|\s*(M\d+)\s*\|\s*(.*?)\s*\|\s*(.*?)\s*\|\s*(.*?)\s*\|\s*$`)
	subMilestone = regexp.MustCompile(`^###\s+(M\d+)\b`)
	doneDate     = regexp.MustCompile(`done (\d{4}-\d{2}-\d{2})`)
	folderRef    = regexp.MustCompile("→ `(?:tasks/)?((?:_archive/)?[A-Z][A-Z0-9]*-[A-Za-z0-9+]+[^/`]*)")
)

// sectionStatus maps a "## " section to a status.
var sectionStatus = map[string]string{
	"Backlog":     "backlog",
	"Ready":       "todo",
	"In Progress": "wip",
	"Review":      "review",
	"Done":        "done",
}

// Parse reads a board.
func Parse(r io.Reader) (*Board, error) {
	b := &Board{}
	status, section, milestone := "", "", ""
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 4<<20)
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "## "):
			name := strings.TrimSpace(strings.TrimPrefix(line, "## "))
			section, milestone = name, ""
			status = sectionStatus[name]
		case strings.HasPrefix(line, "### "):
			section = strings.TrimSpace(strings.TrimPrefix(line, "### "))
			milestone = ""
			if m := subMilestone.FindStringSubmatch(line); m != nil {
				milestone = m[1]
			}
		case milestoneRow.MatchString(line):
			m := milestoneRow.FindStringSubmatch(line)
			b.Milestones = append(b.Milestones, Milestone{ID: m[1], Title: m[2], Spec: m[3], Status: milestoneStatus(m[4]), Note: m[4]})
		case strings.HasPrefix(line, "- ["):
			if status == "" {
				b.Skipped = append(b.Skipped, line)
				continue
			}
			c, ok := parseCard(line, status, section, milestone)
			if !ok {
				b.Skipped = append(b.Skipped, line)
				continue
			}
			b.Cards = append(b.Cards, c)
		}
	}
	return b, sc.Err()
}

// parseCard reads one bullet.
func parseCard(line, status, section, milestone string) (Card, bool) {
	m := cardLine.FindStringSubmatch(line)
	if m == nil {
		return Card{}, false
	}
	var ids []string
	for part := range strings.SplitSeq(m[1], ",") {
		if id := idPart.FindString(strings.TrimSpace(part)); id != "" {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return Card{}, false
	}
	rest := m[2]
	title, _, _ := strings.Cut(rest, " — ")
	title = strings.TrimSpace(title)
	if len(title) > 280 {
		title = strings.TrimSpace(title[:strings.LastIndex(title[:280], " ")]) + " …"
	}
	c := Card{ID: ids[0], Milestone: milestone, Title: title, Status: status, Section: section, Description: strings.TrimPrefix(line, "- ")}
	if len(ids) > 1 {
		c.AlsoIDs = ids[1:]
	}
	if d := doneDate.FindStringSubmatch(line); d != nil {
		if t, err := time.Parse(time.DateOnly, d[1]); err == nil {
			c.DoneOn = t
		}
	}
	if f := folderRef.FindStringSubmatch(line); f != nil {
		c.Folder = "tasks/" + strings.TrimSuffix(f[1], "/")
	}
	lower := strings.ToLower(line)
	switch {
	case strings.Contains(lower, "research") || strings.Contains(lower, "(idea"):
		c.Type = "research"
	case section == "Tech debt (code, tests, UI polish)" || strings.HasPrefix(section, "Tech debt"):
		c.Type = "tech-debt"
	case milestone == "M5":
		c.Type = "security-debt"
	default:
		c.Type = "task"
	}
	c.Guessed = append(c.Guessed, "type")
	if strings.Contains(lower, "waiting for dhira") || strings.Contains(lower, "remind dhira") {
		c.Waiting = "see the imported description"
	}
	return c, true
}

// milestoneStatus maps the milestone table's status cell to a status.
func milestoneStatus(cell string) string {
	switch {
	case strings.Contains(cell, "✅"):
		return "done"
	case strings.Contains(cell, "🔄"):
		return "wip"
	case strings.Contains(cell, "⏭"):
		return "todo"
	default:
		return "backlog"
	}
}
