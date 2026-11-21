package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/Wigata-Intech/w-tools/cli"

	"github.com/mrdhira/camelot/services/camelot/config"
	"github.com/mrdhira/camelot/services/camelot/internal/taskmodel"
)

// optString is a string flag that records whether it was given, so an
// update changes only the fields named on the command line; "" clears.
type optString struct {
	value string
	set   bool
}

func (o *optString) String() string { return o.value }

func (o *optString) Set(v string) error {
	o.value, o.set = v, true
	return nil
}

// ptr returns the value when the flag was given, else nil.
func (o *optString) ptr() *string {
	if !o.set {
		return nil
	}
	return &o.value
}

// list returns the comma-separated value when the flag was given, else nil.
func (o *optString) list() *[]string {
	if !o.set {
		return nil
	}
	l := splitList(o.value)
	return &l
}

// splitList splits a comma-separated list, dropping blanks.
func splitList(v string) []string {
	out := []string{}
	for part := range strings.SplitSeq(v, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// cardFields are the editable card fields as flags.
type cardFields struct {
	milestone, title, cardType, status, assignee, waiting, folder, description, deps, docs optString
}

// declare declares the field flags on fs.
func (f *cardFields) declare(fs *cli.FlagSet) {
	fs.Var(&f.title, "title", "one-line title")
	fs.Var(&f.cardType, "type", "milestone, feature, task, bug, research, tech-debt, feature-debt, product-debt or security-debt")
	fs.Var(&f.status, "status", "backlog, todo, plan, wip, qa, review or done (or the column label)")
	fs.Var(&f.milestone, "milestone", "the milestone card ID; \"\" clears")
	fs.Var(&f.assignee, "assignee", "who works on it (agent name or dhira); \"\" clears")
	fs.Var(&f.waiting, "waiting", "the one-line question waiting for Dhira; \"\" clears")
	fs.Var(&f.folder, "folder", "the card folder under tasks/ (PLAN.md, REPORT.md); \"\" clears")
	fs.Var(&f.description, "description", "Markdown description; \"-\" reads it from stdin")
	fs.Var(&f.deps, "deps", "comma-separated card IDs this card waits for; \"\" clears")
	fs.Var(&f.docs, "docs", "comma-separated document IDs this card builds; \"\" clears")
}

// perCardEnv are the environment variables w-tools cli would bind to the
// per-card flags; a card value never comes from the environment.
var perCardEnv = []string{"CAMELOT_ID", "CAMELOT_PREFIX", "CAMELOT_NOTE", "CAMELOT_UPDATED_AT", "CAMELOT_TITLE", "CAMELOT_TYPE", "CAMELOT_STATUS",
	"CAMELOT_MILESTONE", "CAMELOT_ASSIGNEE", "CAMELOT_WAITING", "CAMELOT_FOLDER", "CAMELOT_DESCRIPTION", "CAMELOT_DEPS", "CAMELOT_DOCS", "CAMELOT_YES"}

// refuseCardEnv fails when any per-card environment variable is set.
func refuseCardEnv() error {
	for _, name := range perCardEnv {
		if _, ok := os.LookupEnv(name); ok {
			return fmt.Errorf("%s is set in the environment; card values come from flags only: unset it", name)
		}
	}
	return nil
}

// readDescription replaces a description of "-" with stdin.
func (f *cardFields) readDescription(stdin io.Reader) error {
	if f.description.set && f.description.value == "-" {
		b, err := io.ReadAll(io.LimitReader(stdin, taskmodel.MaxDescriptionLength+1))
		if err != nil {
			return err
		}
		f.description.value = string(b)
	}
	return nil
}

// taskCommand builds "task" and its subcommands.
func taskCommand(load func() (*config.Config, error), out, logOut io.Writer) *cli.Command {
	return &cli.Command{
		Name:  "task",
		Short: "cards: list, show, create, update, move, delete (JSON output)",
		Commands: []*cli.Command{
			taskListCommand(load, out, logOut),
			taskShowCommand(load, out, logOut),
			taskCreateCommand(load, out, logOut),
			taskUpdateCommand(load, out, logOut, "update", "change the given fields of a card"),
			taskUpdateCommand(load, out, logOut, "move", "move a card to --status (same as update --status)"),
			taskDeleteCommand(load, out, logOut),
		},
	}
}

func taskListCommand(load func() (*config.Config, error), out, logOut io.Writer) *cli.Command {
	var status, cardType, milestone, assignee, doc, query string
	var waiting, withDescription bool
	return &cli.Command{
		Name:  "list",
		Short: "list cards (filters combine)",
		Flags: func(fs *cli.FlagSet) {
			fs.StringVar(&status, "status", "", "comma-separated statuses")
			fs.StringVar(&cardType, "type", "", "comma-separated types")
			fs.StringVar(&milestone, "in-milestone", "", "cards of this milestone")
			fs.StringVar(&assignee, "for", "", "cards assigned to this name")
			fs.StringVar(&doc, "doc", "", "cards linked to this document")
			fs.StringVar(&query, "query", "", "text in the ID or title")
			fs.BoolVar(&waiting, "waiting", false, "only cards waiting on Dhira")
			fs.BoolVar(&withDescription, "with-description", false, "include descriptions")
		},
		Run: runWith(load, logOut, func(ctx context.Context, a *deps) error {
			filter := taskmodel.CardFilter{MilestoneID: milestone, Assignee: assignee, DocID: doc, Waiting: waiting, Query: query}
			for _, s := range splitList(status) {
				st, err := taskmodel.ParseStatus(s)
				if err != nil {
					return err
				}
				filter.Statuses = append(filter.Statuses, st)
			}
			for _, t := range splitList(cardType) {
				ty, err := taskmodel.ParseType(t)
				if err != nil {
					return err
				}
				filter.Types = append(filter.Types, ty)
			}
			cards, err := a.tasks.List(ctx, &taskmodel.ListRequest{Filter: filter})
			if err != nil {
				return err
			}
			if !withDescription {
				for _, c := range cards {
					c.Description = ""
				}
			}
			if cards == nil {
				cards = []*taskmodel.Card{}
			}
			return writeJSON(out, cards)
		}),
	}
}

func taskShowCommand(load func() (*config.Config, error), out, logOut io.Writer) *cli.Command {
	var id string
	return &cli.Command{
		Name:  "show",
		Short: "a card with its dependencies, dependents, children and history",
		Flags: func(fs *cli.FlagSet) {
			fs.StringVar(&id, "id", "", "the card ID")
			fs.Required("id")
		},
		Run: runWith(load, logOut, func(ctx context.Context, a *deps) error {
			card, err := a.tasks.Get(ctx, &taskmodel.GetRequest{ID: id})
			if err != nil {
				return err
			}
			return writeJSON(out, card)
		}),
	}
}

func taskCreateCommand(load func() (*config.Config, error), out, logOut io.Writer) *cli.Command {
	var id, prefix, note string
	var f cardFields
	return &cli.Command{
		Name:  "create",
		Short: "create a card: --id, or --prefix for the next number",
		Flags: func(fs *cli.FlagSet) {
			fs.StringVar(&id, "id", "", "the card ID (WIG-082)")
			fs.StringVar(&prefix, "prefix", "", "take the next number of this prefix (DITA → DITA-011)")
			fs.StringVar(&note, "note", "", "why, recorded with the event")
			f.declare(fs)
		},
		Run: runWith(load, logOut, func(ctx context.Context, a *deps) error {
			if err := f.readDescription(stdinReader); err != nil {
				return err
			}
			request := &taskmodel.CreateRequest{
				ID:          id,
				Prefix:      prefix,
				MilestoneID: f.milestone.value,
				Title:       f.title.value,
				Type:        f.cardType.value,
				Status:      f.status.value,
				Assignee:    f.assignee.value,
				Waiting:     f.waiting.value,
				Folder:      f.folder.value,
				Description: f.description.value,
				Deps:        splitList(f.deps.value),
				Docs:        splitList(f.docs.value),
				Actor:       a.config.Actor,
				Note:        note,
			}
			card, err := a.tasks.Create(ctx, request)
			if err != nil {
				return err
			}
			return writeJSON(out, card)
		}),
	}
}

func taskUpdateCommand(load func() (*config.Config, error), out, logOut io.Writer, name, short string) *cli.Command {
	var id, note, updatedAt string
	var f cardFields
	return &cli.Command{
		Name:  name,
		Short: short,
		Flags: func(fs *cli.FlagSet) {
			fs.StringVar(&id, "id", "", "the card ID")
			fs.Required("id")
			fs.StringVar(&note, "note", "", "why, recorded with each event")
			fs.StringVar(&updatedAt, "updated-at", "", "refuse the change when the card's updated_at differs (RFC 3339, from task show)")
			f.declare(fs)
			if name == "move" {
				fs.Required("status")
			}
		},
		Run: runWith(load, logOut, func(ctx context.Context, a *deps) error {
			if err := f.readDescription(stdinReader); err != nil {
				return err
			}
			request := &taskmodel.UpdateRequest{
				ID:          id,
				MilestoneID: f.milestone.ptr(),
				Title:       f.title.ptr(),
				Type:        f.cardType.ptr(),
				Status:      f.status.ptr(),
				Assignee:    f.assignee.ptr(),
				Waiting:     f.waiting.ptr(),
				Folder:      f.folder.ptr(),
				Description: f.description.ptr(),
				Deps:        f.deps.list(),
				Docs:        f.docs.list(),
				Actor:       a.config.Actor,
				Note:        note,
			}
			if updatedAt != "" {
				t, err := time.Parse(time.RFC3339Nano, updatedAt)
				if err != nil {
					return err
				}
				request.UpdatedAt = t
			}
			card, err := a.tasks.Update(ctx, request)
			if err != nil {
				return err
			}
			return writeJSON(out, card)
		}),
	}
}

func taskDeleteCommand(load func() (*config.Config, error), out, logOut io.Writer) *cli.Command {
	var id, note string
	var yes bool
	return &cli.Command{
		Name:  "delete",
		Short: "delete a card (its history stays); needs --yes",
		Flags: func(fs *cli.FlagSet) {
			fs.StringVar(&id, "id", "", "the card ID")
			fs.Required("id")
			fs.StringVar(&note, "note", "", "why, recorded with the event")
			fs.BoolVar(&yes, "yes", false, "confirm the delete")
		},
		Run: runWith(load, logOut, func(ctx context.Context, a *deps) error {
			if !yes {
				return errNeedsYes
			}
			if err := a.tasks.Delete(ctx, &taskmodel.DeleteRequest{ID: id, Actor: a.config.Actor, Note: note}); err != nil {
				return err
			}
			return writeJSON(out, map[string]string{"deleted": id})
		}),
	}
}
