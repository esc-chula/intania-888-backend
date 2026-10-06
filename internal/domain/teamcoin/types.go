package teamcoin

import "time"

// Event kinds written to the ledger.
const (
	// KindSettled is the first row for a match and color, written when the match result is set.
	KindSettled = "SETTLED"
	// KindAdjusted corrects a settled row after a bill is voided.
	KindAdjusted = "ADJUSTED"
)

// Tally is one color's current bets on a decided match, excluding voided bills.
// Vote counts distinct members per side and drives the team vote; Bets counts
// individual bets and drives the right/wrong accuracy totals.
type Tally struct {
	ColorID   string
	VoteRight int64
	VoteWrong int64
	BetsRight int64
	BetsWrong int64
}

// Balance is the ledger total for one color on one match.
type Balance struct {
	ColorID   string
	Amount    int64
	BetsRight int64
	BetsWrong int64
}

// Event is one ledger row. Deltas are relative to the color's earlier rows for the match.
type Event struct {
	ID             string
	MatchID        string
	ColorID        string
	Kind           string
	BillID         string
	AmountDelta    int64
	BetsRightDelta int64
	BetsWrongDelta int64
	VoteRight      int64
	VoteWrong      int64
	CreatedAt      time.Time
}
