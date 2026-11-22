package app

import (
	"github.com/a-h/templ"
	"github.com/axadrn/shadcn-templ/v2/components/icon"

	"github.com/mrdhira/camelot/services/camelot/internal/docmodel"
	"github.com/mrdhira/camelot/services/camelot/internal/taskmodel"
)

// Tone is one of the six meanings a colour carries (docs/design.md, /design).
type Tone string

// The six meanings.
const (
	ToneYou     Tone = "you"     // your turn: a question waits, a review
	ToneDanger  Tone = "danger"  // something is wrong: blocked, a gap, too old
	ToneActive  Tone = "active"  // an agent's turn: plan, work, QA
	ToneDone    Tone = "done"    // finished: done, approved
	ToneNew     Tone = "new"     // new to you: unread changes
	ToneNeutral Tone = "neutral" // nobody's turn yet: backlog, draft
)

// Tones is every meaning in the order the design book shows them.
var Tones = []Tone{ToneYou, ToneDanger, ToneActive, ToneDone, ToneNew, ToneNeutral}

// toneTints are a tone's tinted look (states).
var toneTints = map[Tone]string{
	ToneYou:     "bg-tone-you-tint text-tone-you",
	ToneDanger:  "bg-tone-danger-tint text-tone-danger",
	ToneActive:  "bg-tone-active-tint text-tone-active",
	ToneDone:    "bg-tone-done-tint text-tone-done",
	ToneNew:     "bg-tone-new-tint text-tone-new",
	ToneNeutral: "bg-tone-neutral-tint text-tone-neutral",
}

// toneFills are the filled ("act now") looks; only these tones have one.
var toneFills = map[Tone]string{
	ToneYou:    "bg-tone-you text-tone-you-on",
	ToneDanger: "bg-tone-danger text-tone-danger-on",
	ToneNew:    "bg-tone-new text-tone-new-on",
}

// toneDots are a tone's colour for a dot beside a label (board columns).
var toneDots = map[Tone]string{
	ToneYou:     "bg-tone-you text-tone-you",
	ToneDanger:  "bg-tone-danger text-tone-danger",
	ToneActive:  "bg-tone-active text-tone-active",
	ToneDone:    "bg-tone-done text-tone-done",
	ToneNew:     "bg-tone-new text-tone-new",
	ToneNeutral: "bg-tone-neutral text-tone-neutral",
}

// toneIcons are a tone's shape, so the meaning never rests on colour alone.
var toneIcons = map[Tone]func(...icon.Props) templ.Component{
	ToneYou:     icon.CircleAlert,
	ToneDanger:  icon.Ban,
	ToneActive:  icon.LoaderCircle,
	ToneDone:    icon.CircleCheck,
	ToneNew:     icon.Sparkles,
	ToneNeutral: icon.CircleDashed,
}

// ToneMeaning is a tone in words, for the design book and the tags' titles.
func ToneMeaning(t Tone) string {
	switch t {
	case ToneYou:
		return "Your turn, act now: a question waits, a card or document is in review."
	case ToneDanger:
		return "Something is wrong: blocked, a document gap, a data problem; tinted when it is a risk (older than usual)."
	case ToneActive:
		return "An agent's turn: Plan, Work and QA in progress; feedback waiting for the agent."
	case ToneDone:
		return "Finished and fine: Done, Approved."
	case ToneNew:
		return "New to you: unread changes."
	case ToneNeutral:
		return "Nobody's turn yet: Backlog, TODO, Draft, Superseded."
	}
	return ""
}

// StageTone is whose turn a card stage is.
func StageTone(s taskmodel.Status) Tone {
	switch s {
	case taskmodel.StatusPlan, taskmodel.StatusWIP, taskmodel.StatusQA:
		return ToneActive
	case taskmodel.StatusReview:
		return ToneYou
	case taskmodel.StatusDone:
		return ToneDone
	}
	return ToneNeutral
}

// DocTone is whose turn a document status is.
func DocTone(s docmodel.Status) Tone {
	switch s {
	case docmodel.StatusInReview:
		return ToneYou
	case docmodel.StatusApproved:
		return ToneDone
	}
	return ToneNeutral
}

// stageDot is a stage's colour class for a dot beside its label.
func stageDot(s taskmodel.Status) string { return toneDots[StageTone(s)] }

// TagProps is a meaning badge: a word, the tone's icon (or Icon) and its
// colour; Filled only for "act now" (needs you, blocked; the unread count).
type TagProps struct {
	Tone   Tone
	Filled bool
	Icon   func(...icon.Props) templ.Component
	Title  string
}

// tagClass is the badge's tone classes: filled when asked and the tone has
// a filled look, else its tint.
func tagClass(p TagProps) string {
	if fill, ok := toneFills[p.Tone]; ok && p.Filled {
		return "border-transparent gap-1 " + fill
	}
	return "border-transparent gap-1 " + toneTints[p.Tone]
}

// tagIcon is the tag's icon.
func tagIcon(p TagProps) templ.Component {
	draw := p.Icon
	if draw == nil {
		draw = toneIcons[p.Tone]
	}
	return draw(icon.Props{Size: 12, Attributes: templ.Attributes{"aria-hidden": "true"}})
}

// toneIf is tone when on, else no tone.
func toneIf(on bool, tone Tone) Tone {
	if on {
		return tone
	}
	return ""
}
