package app

import (
	"context"
	"time"
)

// Shell is what every page's header shows: the unread count of the feed and
// the change stamp live.js compares against.
type Shell struct {
	Unread int
	Stamp  string
	AsOf   time.Time
}

type shellKey struct{}

// WithShell returns ctx carrying the header's facts.
func WithShell(ctx context.Context, s Shell) context.Context {
	return context.WithValue(ctx, shellKey{}, s)
}

// shellOf returns the header's facts; zero when absent.
func shellOf(ctx context.Context) Shell {
	s, _ := ctx.Value(shellKey{}).(Shell)
	return s
}
