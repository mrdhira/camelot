package cmd

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/Wigata-Intech/w-tools/cli"

	"github.com/mrdhira/camelot/services/camelot/config"
	"github.com/mrdhira/camelot/services/camelot/internal/docmodel"
	"github.com/mrdhira/camelot/services/camelot/internal/pkg/sqlitedb"
)

// stdinReader is where "-" values are read from.
var stdinReader io.Reader = os.Stdin

// errNeedsYes refuses a delete without --yes.
var errNeedsYes = errors.New("delete needs --yes")

// docCommand builds "doc" and its subcommands.
func docCommand(load func() (*config.Config, error), out, logOut io.Writer) *cli.Command {
	var docType, status, id, newStatus string
	return &cli.Command{
		Name:  "doc",
		Short: "documents: list, show, status (JSON output)",
		Commands: []*cli.Command{
			{
				Name:  "list",
				Short: "list documents and the problems found in them",
				Flags: func(fs *cli.FlagSet) {
					fs.StringVar(&docType, "type", "", "comma-separated types: okr, kpi, brd, prd, trd")
					fs.StringVar(&status, "status", "", "comma-separated statuses: draft, in-review, approved, superseded")
				},
				Run: runWith(load, logOut, func(ctx context.Context, a *deps) error {
					request := &docmodel.ListRequest{}
					for _, t := range splitList(docType) {
						ty, err := docmodel.ParseType(t)
						if err != nil {
							return err
						}
						request.Types = append(request.Types, ty)
					}
					for _, s := range splitList(status) {
						st, err := docmodel.ParseStatus(s)
						if err != nil {
							return err
						}
						request.Statuses = append(request.Statuses, st)
					}
					list, err := a.docs.List(ctx, request)
					if err != nil {
						return err
					}
					if list.Problems == nil {
						list.Problems = []docmodel.Problem{}
					}
					return writeJSON(out, list)
				}),
			},
			{
				Name:  "show",
				Short: "a document with its chain up and down",
				Flags: func(fs *cli.FlagSet) {
					fs.StringVar(&id, "id", "", "the document ID")
					fs.Required("id")
				},
				Run: runWith(load, logOut, func(ctx context.Context, a *deps) error {
					chain, err := a.docs.Get(ctx, &docmodel.GetRequest{ID: id})
					if err != nil {
						return err
					}
					return writeJSON(out, chain)
				}),
			},
			{
				Name:  "status",
				Short: "set a document's status (its file's frontmatter; updated = today)",
				Flags: func(fs *cli.FlagSet) {
					fs.StringVar(&id, "id", "", "the document ID")
					fs.Required("id")
					fs.StringVar(&newStatus, "status", "", "draft, in-review, approved or superseded")
					fs.Required("status")
				},
				Run: runWith(load, logOut, func(ctx context.Context, a *deps) error {
					doc, err := a.docs.SetStatus(ctx, &docmodel.SetStatusRequest{ID: id, Status: newStatus, Actor: a.config.Actor})
					if err != nil {
						return err
					}
					return writeJSON(out, doc)
				}),
			},
		},
	}
}

// insightsCommand builds "insights".
func insightsCommand(load func() (*config.Config, error), out, logOut io.Writer) *cli.Command {
	return &cli.Command{
		Name:  "insights",
		Short: "the cockpit's numbers as JSON: flow measures, waiting, blocked, stale, agents",
		Run: runWith(load, logOut, func(ctx context.Context, a *deps) error {
			o, err := a.insights.Overview(ctx)
			if err != nil {
				return err
			}
			return writeJSON(out, o.Summary())
		}),
	}
}

// checkCommand builds "check"; it fails when any problem is found.
func checkCommand(load func() (*config.Config, error), out, logOut io.Writer) *cli.Command {
	return &cli.Command{
		Name:  "check",
		Short: "data quality: card links, folders, document frontmatter and chains; exit 1 on any problem",
		Run: runWith(load, logOut, func(ctx context.Context, a *deps) error {
			check, err := a.insights.Check(ctx)
			if err != nil {
				return err
			}
			if err := writeJSON(out, check); err != nil {
				return err
			}
			if len(check.Problems) > 0 {
				return errors.New("check found problems")
			}
			return nil
		}),
	}
}

// backupCommand builds "backup".
func backupCommand(load func() (*config.Config, error), out, logOut io.Writer) *cli.Command {
	return &cli.Command{
		Name:  "backup",
		Short: "write a consistent copy of the database to backups/ beside it now",
		Run: runWith(load, logOut, func(ctx context.Context, a *deps) error {
			dir := filepath.Join(filepath.Dir(a.config.DB), "backups")
			target := filepath.Join(dir, time.Now().UTC().Format("2006-01-02T150405Z")+".db")
			if err := sqlitedb.Backup(ctx, a.db, target); err != nil {
				return err
			}
			if err := pruneBackups(dir); err != nil {
				return err
			}
			return writeJSON(out, map[string]string{"backup": target})
		}),
	}
}
