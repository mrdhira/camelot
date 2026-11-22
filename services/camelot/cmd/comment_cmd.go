package cmd

import (
	"context"
	"io"

	"github.com/Wigata-Intech/w-tools/cli"

	"github.com/mrdhira/camelot/services/camelot/config"
	"github.com/mrdhira/camelot/services/camelot/internal/commentmodel"
)

// commentCommand builds "comment" and its subcommands.
func commentCommand(load func() (*config.Config, error), out, logOut io.Writer) *cli.Command {
	var kind, id, section, body, note string
	var open bool
	var commentID int64
	target := func(fs *cli.FlagSet) {
		fs.StringVar(&kind, "kind", "", "card or doc")
		fs.StringVar(&id, "id", "", "the card or document ID")
		fs.Required("kind")
		fs.Required("id")
	}
	return &cli.Command{
		Name:  "comment",
		Short: "comments on cards and documents: list, add, resolve (JSON output)",
		Commands: []*cli.Command{
			{
				Name:  "list",
				Short: "a card's or document's comments, oldest first",
				Flags: func(fs *cli.FlagSet) {
					target(fs)
					fs.BoolVar(&open, "open", false, "only comments nobody resolved")
				},
				Run: runWith(load, logOut, func(ctx context.Context, a *deps) error {
					thread, err := a.comments.Thread(ctx, &commentmodel.ListRequest{Kind: kind, TargetID: id, OpenOnly: open})
					if err != nil {
						return err
					}
					return writeJSON(out, thread)
				}),
			},
			{
				Name:  "add",
				Short: "comment on a card or document (plain text, at most 4000 characters; - reads stdin)",
				Flags: func(fs *cli.FlagSet) {
					target(fs)
					fs.StringVar(&section, "section", "", "documents only: a heading anchor")
					fs.StringVar(&body, "body", "", "the comment; - reads it from stdin")
					fs.Required("body")
				},
				Run: runWith(load, logOut, func(ctx context.Context, a *deps) error {
					text := body
					if text == "-" {
						b, err := io.ReadAll(io.LimitReader(stdinReader, 4*commentmodel.MaxBody+1))
						if err != nil {
							return err
						}
						text = string(b)
					}
					c, err := a.comments.Add(ctx, &commentmodel.AddRequest{Kind: kind, TargetID: id, Section: section, Body: text, Actor: a.config.Actor})
					if err != nil {
						return err
					}
					return writeJSON(out, c)
				}),
			},
			{
				Name:  "resolve",
				Short: "close a handled comment, with a note",
				Flags: func(fs *cli.FlagSet) {
					fs.Int64Var(&commentID, "comment", 0, "the comment's id")
					fs.StringVar(&note, "note", "", "how it was handled")
					fs.Required("comment")
				},
				Run: runWith(load, logOut, func(ctx context.Context, a *deps) error {
					c, err := a.comments.Resolve(ctx, &commentmodel.ResolveRequest{ID: commentID, Note: note, Actor: a.config.Actor})
					if err != nil {
						return err
					}
					return writeJSON(out, c)
				}),
			},
		},
	}
}
