package bill

import (
	"time"

	"github.com/esc-chula/intania-888-backend/internal/domain/match"
	"github.com/esc-chula/intania-888-backend/internal/value"
)

const (
	// StatusPending marks a bill awaiting match results.
	StatusPending = "PENDING"
	// StatusVoided marks a refunded bill.
	StatusVoided = "VOIDED"
)

// CreateInput supplies a stake and selections to the bill use case.
type CreateInput struct {
	Total value.Money
	Lines []Selection
}

// Selection names a match and the selected team.
type Selection struct {
	MatchID   string
	BettingOn string
}

// Result contains bill state without HTTP or persistence metadata.
type Result struct {
	ID        string
	Total     value.Money
	UserID    string
	Status    string
	Payout    *value.Money
	SettledAt *time.Time
	VoidedAt  *time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
	Lines     []Line
}

// Line contains a bill selection and its snapshotted rate and match.
type Line struct {
	BillID      string
	MatchID     string
	Rate        value.Rate
	BettingOn   string
	VoteColorID *string
	Match       match.Snapshot
}

// BetCount contains the pending selection count for a team.
type BetCount struct {
	BettingOn string
	Count     int64
}

// TerminalEvent records the existing void audit event.
type TerminalEvent struct {
	ID        string
	BillID    string
	Amount    value.Money
	ActorID   string
	Reason    string
	CreatedAt time.Time
}
