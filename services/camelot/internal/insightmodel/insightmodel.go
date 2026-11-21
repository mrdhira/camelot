// Package insightmodel holds the cockpit's numbers: the Kanban flow
// measures (WIP, throughput, work item age, cycle time), what waits for
// Dhira, document progress and gaps. Every number carries its formula and
// sample size.
package insightmodel

import (
	"time"

	"github.com/mrdhira/camelot/services/camelot/internal/docmodel"
	"github.com/mrdhira/camelot/services/camelot/internal/taskmodel"
)

// The thresholds the cockpit flags against.
const (
	// StaleAfter is how long a started card may go without any event.
	StaleAfter = 7 * 24 * time.Hour
	// AgeFallback flags an open card's age while fewer than MinSample cards have a cycle time.
	AgeFallback = 7 * 24 * time.Hour
	// MinSample is the smallest number of cycle times a percentile is shown for.
	MinSample = 5
	// ThroughputWeeks is how many weeks the throughput covers, the current one included.
	ThroughputWeeks = 12
	// ActivityWindow is the window of the agent activity.
	ActivityWindow = 7 * 24 * time.Hour
	// DocStaleAfter is how long a document may go un-updated while its cards move.
	DocStaleAfter = 30 * 24 * time.Hour
)

// The formulas shown beside each number.
const (
	FormulaWIP        = "WIP = cards started but not finished (Plan, WIP, QA, Review); milestones excluded"
	FormulaThroughput = "Throughput = cards whose last move into Done falls in the week (Monday 00:00 UTC); imported cards count at their board done date, never without one"
	FormulaCycleTime  = "Cycle time = last move into Done − first move into Plan/WIP/QA/Review, for Done cards with both; nearest-rank percentiles"
	FormulaAge        = "Work item age = now − first move into Plan/WIP/QA/Review; unknown for cards imported already started"
	FormulaProgress   = "Progress = Done cards ÷ all cards linked to the document or to a document below it in its chain"
	FormulaDebtShare  = "Debt share = open tech, feature, product and security debt cards ÷ all open cards; milestones excluded"
)

// StatusCount is the number of cards in one status.
type StatusCount struct {
	Status taskmodel.Status
	Count  int
}

// TypeCount is the number of cards of one type.
type TypeCount struct {
	Type  taskmodel.Type
	Count int
}

// WeekCount is the number of cards finished in the week starting Start.
type WeekCount struct {
	Start time.Time
	Count int
}

// Percentiles summarizes a sample of durations; zero values with N below MinSample.
type Percentiles struct {
	N   int
	P50 time.Duration
	P85 time.Duration
}

// Shown reports whether the sample is large enough to show.
func (p Percentiles) Shown() bool { return p.N >= MinSample }

// TypeCycle is the cycle time of one card type.
type TypeCycle struct {
	Type  taskmodel.Type
	Cycle Percentiles
}

// AgedCard is an open, started card with its age.
type AgedCard struct {
	Card      *taskmodel.Card
	Age       time.Duration
	Known     bool
	Flagged   bool
	LastEvent *taskmodel.Event
}

// BlockedCard is an unfinished card with dependencies not Done.
type BlockedCard struct {
	Card *taskmodel.Card
	By   []string
}

// MilestoneProgress is a milestone's cards, done and total.
type MilestoneProgress struct {
	Card  *taskmodel.Card
	Done  int
	Total int
}

// AgentActivity is one actor's events in the activity window.
type AgentActivity struct {
	Actor    string    `json:"actor"`
	Events   int       `json:"events"`
	LastAt   time.Time `json:"last_at"`
	LastCard string    `json:"last_card"`
	LastNote string    `json:"last_note"`
}

// Overview is the cockpit and the task insights in one read.
type Overview struct {
	Now          time.Time
	Cards        int
	WIP          int
	ByStatus     []StatusCount
	Throughput   []WeekCount
	Finished     int
	UndatedDone  int
	Cycle        Percentiles
	CycleByType  []TypeCycle
	Open         []TypeCount
	OpenTotal    int
	OpenDebt     int
	Milestones   []MilestoneProgress
	InFlight     []AgedCard
	Waiting      []*taskmodel.Card
	InReview     []*taskmodel.Card
	Blocked      []BlockedCard
	Stale        []AgedCard
	Agents       []AgentActivity
	Recent       []*taskmodel.Event
	DocsInReview []*docmodel.Doc
	Problems     int
}

// DocProgress is one document's linked cards and what is wrong around it.
type DocProgress struct {
	Doc     *docmodel.Doc
	Direct  int
	Done    int
	Total   int
	Gaps    []string
	LastRun time.Time
}

// DocOverview is the document dashboard.
type DocOverview struct {
	Docs     []DocProgress
	Counts   map[docmodel.Type]map[docmodel.Status]int
	Problems []docmodel.Problem
	Gaps     int
}

// Check is the data-quality report: every problem found in cards, links and documents.
type Check struct {
	Problems []CheckProblem `json:"problems"`
}

// CheckProblem is one data-quality finding.
type CheckProblem struct {
	Subject string `json:"subject"`
	Message string `json:"message"`
}

// Summary is the overview as agents read it: durations in days, cards by ID.
type Summary struct {
	AsOf         time.Time           `json:"as_of"`
	Cards        int                 `json:"cards"`
	WIP          SummaryStat         `json:"wip"`
	ByStatus     map[string]int      `json:"by_status"`
	Throughput   SummaryThroughput   `json:"throughput"`
	CycleDays    SummaryCycle        `json:"cycle_time_days"`
	OpenByType   map[string]int      `json:"open_by_type"`
	DebtShare    SummaryStat         `json:"debt_share"`
	InFlight     []SummaryAged       `json:"in_flight"`
	Waiting      []SummaryCard       `json:"waiting_on_dhira"`
	InReview     []string            `json:"in_review"`
	Blocked      map[string][]string `json:"blocked_by"`
	Stale        []string            `json:"stale"`
	Agents       []AgentActivity     `json:"agents_last_7_days"`
	DocsInReview []string            `json:"docs_in_review"`
	DocProblems  int                 `json:"doc_problems"`
}

// SummaryStat is a number with its formula.
type SummaryStat struct {
	Value   float64 `json:"value"`
	N       int     `json:"n"`
	Formula string  `json:"formula"`
}

// SummaryThroughput is the finished cards per week.
type SummaryThroughput struct {
	Weeks       map[string]int `json:"weeks"`
	Total       int            `json:"total"`
	UndatedDone int            `json:"done_without_date_not_counted"`
	Formula     string         `json:"formula"`
}

// SummaryCycle is the cycle-time percentiles in days; null below MinSample.
type SummaryCycle struct {
	N       int      `json:"n"`
	P50     *float64 `json:"p50"`
	P85     *float64 `json:"p85"`
	Formula string   `json:"formula"`
}

// SummaryAged is an in-flight card with its age in days (null when unknown).
type SummaryAged struct {
	ID      string   `json:"id"`
	Status  string   `json:"status"`
	AgeDays *float64 `json:"age_days"`
	Flagged bool     `json:"flagged"`
}

// SummaryCard is a card ID with its question.
type SummaryCard struct {
	ID      string `json:"id"`
	Waiting string `json:"waiting"`
}

// Days returns d in days, rounded to one decimal.
func Days(d time.Duration) float64 {
	return float64(int(d.Hours()/24*10+0.5)) / 10
}

// Summary returns the overview as agents read it.
func (o *Overview) Summary() Summary {
	s := Summary{
		AsOf:         o.Now,
		Cards:        o.Cards,
		WIP:          SummaryStat{Value: float64(o.WIP), N: o.Cards, Formula: FormulaWIP},
		ByStatus:     map[string]int{},
		Throughput:   SummaryThroughput{Weeks: map[string]int{}, Total: o.Finished, Formula: FormulaThroughput},
		CycleDays:    SummaryCycle{N: o.Cycle.N, Formula: FormulaCycleTime},
		OpenByType:   map[string]int{},
		Blocked:      map[string][]string{},
		InFlight:     []SummaryAged{},
		Waiting:      []SummaryCard{},
		InReview:     []string{},
		Stale:        []string{},
		Agents:       o.Agents,
		DocsInReview: []string{},
		DocProblems:  o.Problems,
	}
	for _, c := range o.ByStatus {
		s.ByStatus[string(c.Status)] = c.Count
	}
	for _, w := range o.Throughput {
		s.Throughput.Weeks[w.Start.Format(time.DateOnly)] = w.Count
	}
	if o.Cycle.Shown() {
		p50, p85 := Days(o.Cycle.P50), Days(o.Cycle.P85)
		s.CycleDays.P50, s.CycleDays.P85 = &p50, &p85
	}
	for _, c := range o.Open {
		s.OpenByType[string(c.Type)] = c.Count
	}
	if o.OpenTotal > 0 {
		s.DebtShare = SummaryStat{Value: float64(int(float64(o.OpenDebt)/float64(o.OpenTotal)*1000+0.5)) / 1000, N: o.OpenTotal, Formula: FormulaDebtShare}
	} else {
		s.DebtShare = SummaryStat{Formula: FormulaDebtShare}
	}
	for _, a := range o.InFlight {
		aged := SummaryAged{ID: a.Card.ID, Status: string(a.Card.Status), Flagged: a.Flagged}
		if a.Known {
			d := Days(a.Age)
			aged.AgeDays = &d
		}
		s.InFlight = append(s.InFlight, aged)
	}
	for _, c := range o.Waiting {
		s.Waiting = append(s.Waiting, SummaryCard{ID: c.ID, Waiting: c.Waiting})
	}
	for _, c := range o.InReview {
		s.InReview = append(s.InReview, c.ID)
	}
	for _, b := range o.Blocked {
		s.Blocked[b.Card.ID] = b.By
	}
	for _, a := range o.Stale {
		s.Stale = append(s.Stale, a.Card.ID)
	}
	for _, d := range o.DocsInReview {
		s.DocsInReview = append(s.DocsInReview, d.ID)
	}
	if s.Agents == nil {
		s.Agents = []AgentActivity{}
	}
	return s
}
