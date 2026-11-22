// Package searchmodel holds search results over cards and documents.
package searchmodel

// The kinds of result.
const (
	KindCard = "card"
	KindDoc  = "doc"
)

// MaxResults bounds a search's results.
const MaxResults = 20

// Result is one match.
type Result struct {
	Kind   string
	ID     string
	Title  string
	Detail string
}

// Request searches for Query, case-insensitively; an empty query lists the most recently changed cards.
type Request struct {
	Query string
}
