// Package app holds Camelot's pages: templ views over the services'
// models, with the formatting they share.
package app

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/axadrn/shadcn-templ/v2/components/badge"

	"github.com/mrdhira/camelot/services/camelot/internal/docmodel"
	"github.com/mrdhira/camelot/services/camelot/internal/taskmodel"
)

// Zone is the time zone pages show times in; set once at startup.
var Zone = time.UTC

// Duration renders a duration for people: minutes under an hour, hours
// under two days, else days with one decimal.
func Duration(d time.Duration) string {
	switch {
	case d < time.Hour:
		return fmt.Sprintf("%d min", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%.0f h", d.Hours())
	default:
		return fmt.Sprintf("%.1f d", d.Hours()/24)
	}
}

// Ago renders how long before now t was.
func Ago(t, now time.Time) string {
	if t.IsZero() {
		return "never"
	}
	return Duration(now.Sub(t)) + " ago"
}

// When renders t in Zone, to the minute, with the zone's name.
func When(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.In(Zone).Format("2006-01-02 15:04 MST")
}

// Day renders t's date in Zone.
func Day(t time.Time) string {
	return t.In(Zone).Format("Jan 2")
}

// ISO renders t as RFC 3339 for a <time> element and hidden form fields.
func ISO(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

// Percent renders part of total as a whole percentage.
func Percent(part, total int) string {
	if total == 0 {
		return "0%"
	}
	return fmt.Sprintf("%d%%", part*100/total)
}

// StatusVariant is the badge look of a card status.
func StatusVariant(s taskmodel.Status) badge.Variant {
	switch s {
	case taskmodel.StatusDone:
		return badge.VariantSecondary
	case taskmodel.StatusBacklog, taskmodel.StatusTodo:
		return badge.VariantOutline
	default:
		return badge.VariantDefault
	}
}

// DocStatusVariant is the badge look of a document status.
func DocStatusVariant(s docmodel.Status) badge.Variant {
	switch s {
	case docmodel.StatusApproved:
		return badge.VariantDefault
	case docmodel.StatusInReview:
		return badge.VariantSecondary
	default:
		return badge.VariantOutline
	}
}

// TaskHref is a card's page.
func TaskHref(id string) string { return "/tasks/" + id }

// DocHref is a document's page.
func DocHref(id string) string { return "/docs/" + id }

// statusTitle is an error page's title.
func statusTitle(status int) string {
	if status == 0 {
		return "Error"
	}
	return fmt.Sprintf("%d %s", status, http.StatusText(status))
}

// itoa formats an int.
func itoa(n int) string { return fmt.Sprint(n) }

// plural renders n with its noun.
func plural(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return fmt.Sprintf("%d %s", n, many)
}

// joinIDs joins IDs with commas, for form fields.
func joinIDs(ids []string) string { return strings.Join(ids, ", ") }

// percentOf is a bar's height or width in percent of max.
func percentOf(v, maxV int) float64 {
	if maxV == 0 {
		return 0
	}
	return float64(v) * 100 / float64(maxV)
}
