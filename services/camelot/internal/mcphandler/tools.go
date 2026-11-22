package mcphandler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mrdhira/camelot/services/camelot/internal/commentmodel"
	"github.com/mrdhira/camelot/services/camelot/internal/commentservice"
	"github.com/mrdhira/camelot/services/camelot/internal/docmodel"
	"github.com/mrdhira/camelot/services/camelot/internal/docservice"
	"github.com/mrdhira/camelot/services/camelot/internal/insightservice"
	"github.com/mrdhira/camelot/services/camelot/internal/taskmodel"
	"github.com/mrdhira/camelot/services/camelot/internal/taskservice"
)

// tool is one MCP tool.
type tool struct {
	name        string
	title       string
	description string
	schema      map[string]any
	annotations map[string]any
	run         func(ctx context.Context, args json.RawMessage) (any, error)
}

// Schema helpers.
func str(description string) map[string]any {
	return map[string]any{"type": "string", "description": description}
}

func strList(description string) map[string]any {
	return map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": description}
}

func object(props map[string]any, required ...string) map[string]any {
	o := map[string]any{"type": "object", "properties": props, "additionalProperties": false}
	if len(required) > 0 {
		o["required"] = required
	}
	return o
}

// readOnly, writes and destructive are the tools' behaviour hints.
var (
	readOnly    = map[string]any{"readOnlyHint": true, "destructiveHint": false, "openWorldHint": false}
	writes      = map[string]any{"readOnlyHint": false, "destructiveHint": false, "idempotentHint": false, "openWorldHint": false}
	destructive = map[string]any{"readOnlyHint": false, "destructiveHint": true, "openWorldHint": false}
)

// cardProps are the editable card fields.
func cardProps() map[string]any {
	return map[string]any{
		"title":           str("one-line title"),
		"type":            str("milestone, feature, task, bug, research, tech-debt, feature-debt, product-debt or security-debt"),
		"status":          str("backlog, todo, plan, wip, qa, review or done"),
		"milestone":       str("milestone card ID; empty clears"),
		"assignee":        str("who works on it: your agent name, or dhira; empty clears"),
		"waiting":         str("one-line question waiting for Dhira's decision, with your recommendation; empty clears"),
		"waiting_options": strList("answer options shown as buttons, the recommended first (at most 6); Dhira's answer comes back as a decision event on the card (get_task)"),
		"folder":          str("card folder under tasks/ holding PLAN.md and REPORT.md; empty clears"),
		"description":     str("Markdown description"),
		"deps":            strList("card IDs this card waits for (replaces the list)"),
		"docs":            strList("document IDs this card builds, such as TRD003 (replaces the list)"),
		"note":            str("why, recorded with the change"),
		"actor":           str("your agent name (go-engineer, qa-ui-engineer, dita); default from CAMELOT_ACTOR"),
	}
}

// with returns props plus extra.
func with(props map[string]any, extra map[string]any) map[string]any {
	for k, v := range extra {
		props[k] = v
	}
	return props
}

// decode reads args strictly into v.
func decode(args json.RawMessage, v any) error {
	dec := json.NewDecoder(bytes.NewReader(args))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("invalid arguments: %w", err)
	}
	return nil
}

// newTools builds the tool list, in a fixed order.
func newTools(config Config, taskSvc taskservice.Service, docSvc docservice.Service, insightSvc insightservice.Service, commentSvc commentservice.Service) []tool {
	actor := func(a string) string {
		if a == "" {
			return config.Actor
		}
		return a
	}
	type cardArgs struct {
		ID          string    `json:"id"`
		Prefix      string    `json:"prefix"`
		Title       *string   `json:"title"`
		Type        *string   `json:"type"`
		Status      *string   `json:"status"`
		Milestone   *string   `json:"milestone"`
		Assignee    *string   `json:"assignee"`
		Waiting     *string   `json:"waiting"`
		Options     *[]string `json:"waiting_options"`
		Folder      *string   `json:"folder"`
		Description *string   `json:"description"`
		Deps        *[]string `json:"deps"`
		Docs        *[]string `json:"docs"`
		Note        string    `json:"note"`
		Actor       string    `json:"actor"`
		UpdatedAt   string    `json:"updated_at"`
	}
	value := func(p *string) string {
		if p == nil {
			return ""
		}
		return *p
	}
	values := func(p *[]string) []string {
		if p == nil {
			return nil
		}
		return *p
	}
	return []tool{
		{
			name:        "list_tasks",
			title:       "List cards",
			description: "List cards (without descriptions). Filters combine.",
			schema: object(map[string]any{
				"status":    strList("statuses to include"),
				"type":      strList("types to include"),
				"milestone": str("cards of this milestone"),
				"assignee":  str("cards assigned to this name"),
				"doc":       str("cards linked to this document ID"),
				"waiting":   map[string]any{"type": "boolean", "description": "only cards waiting on Dhira"},
				"query":     str("text in the ID or title"),
			}),
			annotations: readOnly,
			run: func(ctx context.Context, args json.RawMessage) (any, error) {
				var a struct {
					Status    []string `json:"status"`
					Type      []string `json:"type"`
					Milestone string   `json:"milestone"`
					Assignee  string   `json:"assignee"`
					Doc       string   `json:"doc"`
					Waiting   bool     `json:"waiting"`
					Query     string   `json:"query"`
				}
				if err := decode(args, &a); err != nil {
					return nil, err
				}
				filter := taskmodel.CardFilter{MilestoneID: a.Milestone, Assignee: a.Assignee, DocID: a.Doc, Waiting: a.Waiting, Query: a.Query}
				for _, s := range a.Status {
					st, err := taskmodel.ParseStatus(s)
					if err != nil {
						return nil, err
					}
					filter.Statuses = append(filter.Statuses, st)
				}
				for _, t := range a.Type {
					ty, err := taskmodel.ParseType(t)
					if err != nil {
						return nil, err
					}
					filter.Types = append(filter.Types, ty)
				}
				cards, err := taskSvc.List(ctx, &taskmodel.ListRequest{Filter: filter})
				if err != nil {
					return nil, err
				}
				for _, c := range cards {
					c.Description = ""
				}
				if cards == nil {
					cards = []*taskmodel.Card{}
				}
				return map[string]any{"cards": cards}, nil
			},
		},
		{
			name:        "get_task",
			title:       "Show a card",
			description: "A card with its description, dependencies (with status), dependents, child cards, full history and open comments. Read the open comments before working on the card: awaits_reply means Dhira wrote last and expects your reply (add_comment) or a fix (then resolve_comment).",
			schema:      object(map[string]any{"id": str("the card ID")}, "id"),
			annotations: readOnly,
			run: func(ctx context.Context, args json.RawMessage) (any, error) {
				var a struct {
					ID string `json:"id"`
				}
				if err := decode(args, &a); err != nil {
					return nil, err
				}
				detail, err := taskSvc.Get(ctx, &taskmodel.GetRequest{ID: a.ID})
				if err != nil {
					return nil, err
				}
				thread, err := commentSvc.Thread(ctx, &commentmodel.ListRequest{Kind: commentmodel.KindCard, TargetID: detail.Card.ID, OpenOnly: true})
				if err != nil {
					return nil, err
				}
				return struct {
					*taskmodel.GetResponse
					Comments *commentmodel.Thread `json:"comments"`
				}{detail, thread}, nil
			},
		},
		{
			name:        "create_task",
			title:       "Create a card",
			description: "Create a card. Give id (WIG-082) or prefix (DITA → the next free number). Defaults: type task, status backlog.",
			schema:      object(with(cardProps(), map[string]any{"id": str("the card ID"), "prefix": str("take the next number of this prefix")}), "title"),
			annotations: writes,
			run: func(ctx context.Context, args json.RawMessage) (any, error) {
				var a cardArgs
				if err := decode(args, &a); err != nil {
					return nil, err
				}
				return taskSvc.Create(ctx, &taskmodel.CreateRequest{
					ID:          a.ID,
					Prefix:      a.Prefix,
					MilestoneID: value(a.Milestone),
					Title:       value(a.Title),
					Type:        value(a.Type),
					Status:      value(a.Status),
					Assignee:    value(a.Assignee),
					Waiting:     value(a.Waiting),
					Options:     values(a.Options),
					Folder:      value(a.Folder),
					Description: value(a.Description),
					Deps:        values(a.Deps),
					Docs:        values(a.Docs),
					Actor:       actor(a.Actor),
					Note:        a.Note,
				})
			},
		},
		{
			name:        "update_task",
			title:       "Update or move a card",
			description: "Change only the fields given; moving is a status change. Each changed field is recorded as an event with note and actor. Pass updated_at from get_task to refuse the change if someone changed the card since.",
			schema:      object(with(cardProps(), map[string]any{"id": str("the card ID"), "updated_at": str("RFC 3339 updated_at the change is based on")}), "id"),
			annotations: writes,
			run: func(ctx context.Context, args json.RawMessage) (any, error) {
				var a cardArgs
				if err := decode(args, &a); err != nil {
					return nil, err
				}
				request := &taskmodel.UpdateRequest{
					ID:          a.ID,
					MilestoneID: a.Milestone,
					Title:       a.Title,
					Type:        a.Type,
					Status:      a.Status,
					Assignee:    a.Assignee,
					Waiting:     a.Waiting,
					Options:     a.Options,
					Folder:      a.Folder,
					Description: a.Description,
					Deps:        a.Deps,
					Docs:        a.Docs,
					Actor:       actor(a.Actor),
					Note:        a.Note,
				}
				if a.UpdatedAt != "" {
					t, err := time.Parse(time.RFC3339Nano, a.UpdatedAt)
					if err != nil {
						return nil, fmt.Errorf("updated_at: %w", err)
					}
					request.UpdatedAt = t
				}
				return taskSvc.Update(ctx, request)
			},
		},
		{
			name:        "delete_task",
			title:       "Delete a card",
			description: "Delete a card nothing depends on and no card belongs to. Its history stays. Prefer moving to done; delete only mistakes.",
			schema:      object(map[string]any{"id": str("the card ID"), "note": str("why"), "actor": str("your agent name")}, "id", "note"),
			annotations: destructive,
			run: func(ctx context.Context, args json.RawMessage) (any, error) {
				var a struct {
					ID    string `json:"id"`
					Note  string `json:"note"`
					Actor string `json:"actor"`
				}
				if err := decode(args, &a); err != nil {
					return nil, err
				}
				if err := taskSvc.Delete(ctx, &taskmodel.DeleteRequest{ID: a.ID, Actor: actor(a.Actor), Note: a.Note}); err != nil {
					return nil, err
				}
				return map[string]any{"deleted": a.ID}, nil
			},
		},
		{
			name:        "list_docs",
			title:       "List documents",
			description: "BRD, PRD, TRD, OKR and KPI documents (from their files' frontmatter), with every problem found in them.",
			schema:      object(map[string]any{"type": strList("okr, kpi, brd, prd, trd"), "status": strList("draft, in-review, approved, superseded")}),
			annotations: readOnly,
			run: func(ctx context.Context, args json.RawMessage) (any, error) {
				var a struct {
					Type   []string `json:"type"`
					Status []string `json:"status"`
				}
				if err := decode(args, &a); err != nil {
					return nil, err
				}
				request := &docmodel.ListRequest{}
				for _, t := range a.Type {
					ty, err := docmodel.ParseType(t)
					if err != nil {
						return nil, err
					}
					request.Types = append(request.Types, ty)
				}
				for _, s := range a.Status {
					st, err := docmodel.ParseStatus(s)
					if err != nil {
						return nil, err
					}
					request.Statuses = append(request.Statuses, st)
				}
				list, err := docSvc.List(ctx, request)
				if err != nil {
					return nil, err
				}
				if list.Problems == nil {
					list.Problems = []docmodel.Problem{}
				}
				return list, nil
			},
		},
		{
			name:        "get_doc",
			title:       "Show a document",
			description: "A document's fields and its chain: what it serves (up) and what serves it (down). With include_body, its Markdown body too.",
			schema:      object(map[string]any{"id": str("the document ID"), "include_body": map[string]any{"type": "boolean"}}, "id"),
			annotations: readOnly,
			run: func(ctx context.Context, args json.RawMessage) (any, error) {
				var a struct {
					ID          string `json:"id"`
					IncludeBody bool   `json:"include_body"`
				}
				if err := decode(args, &a); err != nil {
					return nil, err
				}
				chain, err := docSvc.Get(ctx, &docmodel.GetRequest{ID: a.ID})
				if err != nil {
					return nil, err
				}
				out := map[string]any{"doc": chain.Doc, "up": chain.Up, "down": chain.Down}
				if a.IncludeBody {
					out["body"] = string(chain.Doc.Body)
				}
				return out, nil
			},
		},
		{
			name:        "set_doc_status",
			title:       "Set a document's status",
			description: "Set draft, in-review, approved or superseded in the document file's frontmatter (updated = today). Approved and superseded are Dhira's decisions: set them only on his word.",
			schema:      object(map[string]any{"id": str("the document ID"), "status": str("draft, in-review, approved or superseded"), "actor": str("your agent name")}, "id", "status"),
			annotations: writes,
			run: func(ctx context.Context, args json.RawMessage) (any, error) {
				var a struct {
					ID     string `json:"id"`
					Status string `json:"status"`
					Actor  string `json:"actor"`
				}
				if err := decode(args, &a); err != nil {
					return nil, err
				}
				return docSvc.SetStatus(ctx, &docmodel.SetStatusRequest{ID: a.ID, Status: a.Status, Actor: actor(a.Actor)})
			},
		},
		{
			name:        "list_comments",
			title:       "Read comments",
			description: "The comments on a card or document, oldest first, with who resolved each. A document comment may name a section (a heading anchor).",
			schema: object(map[string]any{
				"kind":      str("card or doc"),
				"id":        str("the card or document ID"),
				"open_only": map[string]any{"type": "boolean", "description": "only comments nobody resolved"},
			}, "kind", "id"),
			annotations: readOnly,
			run: func(ctx context.Context, args json.RawMessage) (any, error) {
				var a struct {
					Kind     string `json:"kind"`
					ID       string `json:"id"`
					OpenOnly bool   `json:"open_only"`
				}
				if err := decode(args, &a); err != nil {
					return nil, err
				}
				return commentSvc.Thread(ctx, &commentmodel.ListRequest{Kind: a.Kind, TargetID: a.ID, OpenOnly: a.OpenOnly})
			},
		},
		{
			name:        "add_comment",
			title:       "Comment",
			description: "Comment on a card or document: answer Dhira's feedback, say what you changed, or ask. Decisions that are Dhira's go in waiting/waiting_options on the card instead.",
			schema: object(map[string]any{
				"kind":    str("card or doc"),
				"id":      str("the card or document ID"),
				"section": str("documents only: a heading anchor of the document (list_comments shows the ones in use)"),
				"body":    str("plain text, at most 4000 characters"),
				"actor":   str("your agent name"),
			}, "kind", "id", "body"),
			annotations: writes,
			run: func(ctx context.Context, args json.RawMessage) (any, error) {
				var a struct {
					Kind    string `json:"kind"`
					ID      string `json:"id"`
					Section string `json:"section"`
					Body    string `json:"body"`
					Actor   string `json:"actor"`
				}
				if err := decode(args, &a); err != nil {
					return nil, err
				}
				return commentSvc.Add(ctx, &commentmodel.AddRequest{Kind: a.Kind, TargetID: a.ID, Section: a.Section, Body: a.Body, Actor: actor(a.Actor)})
			},
		},
		{
			name:        "resolve_comment",
			title:       "Resolve a comment",
			description: "Close a comment once it is handled, with a note saying how. Resolving it again changes nothing.",
			schema: object(map[string]any{
				"comment": map[string]any{"type": "integer", "description": "the comment's id"},
				"note":    str("how it was handled"),
				"actor":   str("your agent name"),
			}, "comment"),
			annotations: writes,
			run: func(ctx context.Context, args json.RawMessage) (any, error) {
				var a struct {
					Comment int64  `json:"comment"`
					Note    string `json:"note"`
					Actor   string `json:"actor"`
				}
				if err := decode(args, &a); err != nil {
					return nil, err
				}
				return commentSvc.Resolve(ctx, &commentmodel.ResolveRequest{ID: a.Comment, Note: a.Note, Actor: actor(a.Actor)})
			},
		},
		{
			name:        "insights",
			title:       "Cockpit numbers",
			description: "WIP, throughput per week, cycle time p50/p85, work item age, debt share, what waits on Dhira, blocked and stale cards, agent activity. Every number with its formula and sample size.",
			schema:      object(map[string]any{}),
			annotations: readOnly,
			run: func(ctx context.Context, args json.RawMessage) (any, error) {
				if err := decode(args, &struct{}{}); err != nil {
					return nil, err
				}
				o, err := insightSvc.Overview(ctx)
				if err != nil {
					return nil, err
				}
				return o.Summary(), nil
			},
		},
		{
			name:        "check",
			title:       "Data-quality check",
			description: "Every problem in cards, links, folders and document frontmatter. Run after bulk changes.",
			schema:      object(map[string]any{}),
			annotations: readOnly,
			run: func(ctx context.Context, args json.RawMessage) (any, error) {
				if err := decode(args, &struct{}{}); err != nil {
					return nil, err
				}
				return insightSvc.Check(ctx)
			},
		},
	}
}
