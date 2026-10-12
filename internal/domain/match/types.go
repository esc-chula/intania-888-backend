package match

import (
	"time"

	"github.com/esc-chula/intania-888-backend/internal/value"
)

// Snapshot is a persisted match state without transport or ORM metadata.
type Snapshot struct {
	ID            string
	TeamAID       *string
	TeamBID       *string
	TeamAScore    *int
	TeamBScore    *int
	WinnerID      *string
	TypeID        string
	LocationID    string
	LocationTitle string
	IsDraw        bool
	StartTime     time.Time
	EndTime       time.Time
}

// Input supplies match details to create and update use cases.
type Input struct {
	ID         string
	TeamAID    string
	TeamBID    string
	TeamAScore *int
	TeamBScore *int
	WinnerID   string
	TypeID     string
	LocationID string
	IsDraw     bool
	StartTime  time.Time
	EndTime    time.Time
}

// Result contains a match and its current authoritative rates.
type Result struct {
	ID            string
	TeamAID       string
	TeamBID       string
	TeamAScore    *int
	TeamBScore    *int
	TeamARate     value.Rate
	TeamBRate     value.Rate
	WinnerID      string
	TypeID        string
	LocationID    string
	LocationTitle string
	IsDraw        bool
	StartTime     time.Time
	EndTime       time.Time
}

// ScoreInput supplies the two non-negative scores.
type ScoreInput struct {
	TeamAScore int
	TeamBScore int
}

// ResultInput declares the terminal winner or draw outcome.
type ResultInput struct {
	Outcome  string
	WinnerID *string
}

// ScheduleFilter chooses matches by end time and recorded completion.
type ScheduleFilter string

const (
	// Schedule selects unfinished matches whose end time is in the future.
	Schedule ScheduleFilter = "schedule"
	// ScheduleResult selects matches that have ended or have a recorded winner or draw.
	ScheduleResult ScheduleFilter = "result"
)

// Filter restricts the list of matches.
type Filter struct {
	TypeID   string
	Schedule ScheduleFilter
}

// BillSnapshot contains the bill state needed for settlement.
type BillSnapshot struct {
	ID     string
	UserID string
	Total  value.Money
	Status string
}

// BillLineSnapshot associates a stored rate and selection with a match.
type BillLineSnapshot struct {
	BillID    string
	MatchID   string
	Rate      value.Rate
	BettingOn string
	Match     Snapshot
}

// UserBalance contains a balance read while its account row is locked.
type UserBalance struct {
	ID      string
	Balance value.Money
}

// BillSettlement records the terminal state written by match settlement.
type BillSettlement struct {
	BillID    string
	Status    string
	Payout    value.Money
	SettledAt time.Time
}

// TerminalEvent records the existing bill audit event.
type TerminalEvent struct {
	ID        string
	BillID    string
	Amount    value.Money
	CreatedAt time.Time
}
