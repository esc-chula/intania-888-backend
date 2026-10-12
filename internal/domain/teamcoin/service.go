package teamcoin

import (
	"context"
	"sort"
	"time"

	"github.com/esc-chula/intania-888-backend/internal/value"
)

// Service records the team coin ledger. It owns the fixed award per won vote,
// the clock, and the ID source, so callers only say which match changed.
type Service struct {
	award int64
	now   func() time.Time
	newID func() string
}

// NewService constructs the team coin service paying award for each match a color wins by vote.
func NewService(award value.Money, now func() time.Time, newID func() string) *Service {
	return &Service{award: award.MinorUnits(), now: now, newID: newID}
}

// Settle records the vote result of a match that was just decided. Call it inside
// the transaction that stores the result.
func (s *Service) Settle(ctx context.Context, repo Repository, matchID string) error {
	return s.reconcile(ctx, repo, matchID, KindSettled, "")
}

// Adjust corrects a decided match's ledger after billID was voided, inside the
// transaction that voids it. It records nothing when the void changes no color's
// result or counts, and nothing for a match that has no result yet.
func (s *Service) Adjust(ctx context.Context, repo Repository, matchID, billID string) error {
	if billID == "" {
		return ErrInvalidInput
	}

	return s.reconcile(ctx, repo, matchID, KindAdjusted, billID)
}

// reconcile brings the ledger for one match in line with the current bets. It
// appends only the difference, so it records nothing when the ledger is already
// correct and is safe to repeat.
func (s *Service) reconcile(ctx context.Context, repo Repository, matchID, kind, billID string) error {
	if matchID == "" || s.now == nil || s.newID == nil {
		return ErrInvalidInput
	}

	tallies, err := repo.TeamVoteTallies(ctx, matchID)
	if err != nil {
		return err
	}
	balances, err := repo.TeamCoinBalances(ctx, matchID)
	if err != nil {
		return err
	}

	events := Plan(tallies, balances, s.award)
	if len(events) == 0 {
		return nil
	}

	now := s.now()
	for i := range events {
		events[i].ID = s.newID()
		events[i].MatchID = matchID
		events[i].Kind = kind
		events[i].BillID = billID
		events[i].CreatedAt = now
	}

	return repo.CreateTeamCoinEvents(ctx, events)
}

// Plan returns the events needed to move the balances to what the tallies imply,
// ordered by color. A color wins the vote when more of its members' money-backed
// votes were right than wrong; a tie, or no remaining bettors, earns nothing.
// Colors that only appear in the balances are brought back to zero.
func Plan(tallies []Tally, balances []Balance, award int64) []Event {
	wanted := make(map[string]Tally, len(tallies))
	colors := make(map[string]struct{}, len(tallies)+len(balances))
	for _, tally := range tallies {
		wanted[tally.ColorID] = tally
		colors[tally.ColorID] = struct{}{}
	}
	have := make(map[string]Balance, len(balances))
	for _, balance := range balances {
		have[balance.ColorID] = balance
		colors[balance.ColorID] = struct{}{}
	}

	ids := make([]string, 0, len(colors))
	for id := range colors {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	var events []Event
	for _, id := range ids {
		tally := wanted[id]
		balance := have[id]

		var amount int64
		if tally.VoteRight > tally.VoteWrong {
			amount = award
		}
		event := Event{
			ColorID:        id,
			AmountDelta:    amount - balance.Amount,
			BetsRightDelta: tally.BetsRight - balance.BetsRight,
			BetsWrongDelta: tally.BetsWrong - balance.BetsWrong,
			VoteRight:      tally.VoteRight,
			VoteWrong:      tally.VoteWrong,
		}
		if event.AmountDelta == 0 && event.BetsRightDelta == 0 && event.BetsWrongDelta == 0 {
			continue
		}
		events = append(events, event)
	}

	return events
}
