package match

import (
	"context"
	"errors"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/esc-chula/intania-888-backend/internal/domain/teamcoin"
	"github.com/esc-chula/intania-888-backend/internal/value"
)

func TestRateForLargeCounts(t *testing.T) {
	rate, err := rateFor(math.MaxInt64, math.MaxInt64, true)

	if err != nil {
		t.Fatal(err)
	}

	if rate.MicroUnits() != 2_000_000 {
		t.Fatalf("rate = %d; want 2000000", rate.MicroUnits())
	}
}

func TestRateForRejectsInvalidAndOverflowingCounts(t *testing.T) {
	if _, err := rateFor(-1, 0, true); err == nil {
		t.Fatal("negative count must be rejected")
	}

	if _, err := rateFor(math.MaxInt64, 0, false); err == nil {
		t.Fatal("unrepresentable rate must be rejected")
	}
}

type settlementManagerFake struct {
	transaction TransactionRepository
	team        teamcoin.Repository
	context     context.Context
}

func (f *settlementManagerFake) WithinTransaction(ctx context.Context, callback func(TransactionRepository, teamcoin.Repository) error) error {
	f.context = ctx

	return callback(f.transaction, f.team)
}

type settlementRepositoryFake struct {
	TransactionRepository
	match          Snapshot
	discoveryCalls int
	resultWrites   int
	settlements    []BillSettlement
	events         []TerminalEvent
	credited       value.Money
	tallies        []teamcoin.Tally
	teamCoinEvents []teamcoin.Event
}

func (f *settlementRepositoryFake) AcquireLifecycleLock(context.Context) error {
	return nil
}

func (f *settlementRepositoryFake) FindPendingBillIDs(context.Context, string) ([]string, error) {
	f.discoveryCalls++

	return []string{"bill"}, nil
}

func (f *settlementRepositoryFake) FindReferencedMatchIDs(context.Context, []string) ([]string, error) {
	return []string{"M"}, nil
}

func (f *settlementRepositoryFake) LockMatches(context.Context, []string) ([]Snapshot, error) {
	return []Snapshot{f.match}, nil
}

func (f *settlementRepositoryFake) UpdateResult(_ context.Context, item *Snapshot, _ time.Time) error {
	f.resultWrites++
	f.match = *item

	return nil
}

func (f *settlementRepositoryFake) LockBills(context.Context, []string) ([]*BillSnapshot, error) {
	return []*BillSnapshot{{
		ID:     "bill",
		UserID: "user",
		Total:  value.MustMoneyFromMinor(10000),
		Status: "PENDING",
	}}, nil
}

func (f *settlementRepositoryFake) FindBillLines(context.Context, []string) ([]BillLineSnapshot, error) {
	return []BillLineSnapshot{{
		BillID:    "bill",
		MatchID:   "M",
		Rate:      value.MustRateFromMicro(2000000),
		BettingOn: "A",
		Match:     f.match,
	}}, nil
}

func (f *settlementRepositoryFake) LockUsers(context.Context, []string) ([]UserBalance, error) {
	return []UserBalance{{
		ID:      "user",
		Balance: value.MustMoneyFromMinor(5000),
	}}, nil
}

func (f *settlementRepositoryFake) UpdateBalance(_ context.Context, _ string, balance value.Money) error {
	f.credited = balance

	return nil
}

func (f *settlementRepositoryFake) SettleBill(_ context.Context, item BillSettlement) error {
	f.settlements = append(f.settlements, item)

	return nil
}

func (f *settlementRepositoryFake) TeamVoteTallies(context.Context, string) ([]teamcoin.Tally, error) {
	return f.tallies, nil
}

func (f *settlementRepositoryFake) TeamCoinBalances(context.Context, string) ([]teamcoin.Balance, error) {
	return nil, nil
}

func (f *settlementRepositoryFake) CreateTeamCoinEvents(_ context.Context, events []teamcoin.Event) error {
	f.teamCoinEvents = append(f.teamCoinEvents, events...)

	return nil
}

func (f *settlementRepositoryFake) CreateTerminalEvent(_ context.Context, item TerminalEvent) error {
	f.events = append(f.events, item)

	return nil
}

func TestSetResultDrawRefundsStakeAndIsIdempotent(t *testing.T) {
	a, b := "A", "B"
	repo := &settlementRepositoryFake{match: Snapshot{
		ID:      "M",
		TeamAID: &a,
		TeamBID: &b,
	}}
	manager := &settlementManagerFake{transaction: repo, team: repo}
	now := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	newID := func() string { return "audit" }
	service := NewService(nil, manager, clock, newID,
		teamcoin.NewService(value.MustMoneyFromMinor(100_00), clock, newID))
	ctx := context.WithValue(context.Background(), contextKey{}, "request")
	if err := service.SetResult(ctx, "M", &ResultInput{Outcome: "draw"}); err != nil {
		t.Fatal(err)
	}
	if manager.context != ctx {
		t.Fatal("request context was not propagated")
	}
	if repo.credited.MinorUnits() != 15000 {
		t.Fatalf("balance = %s; want 150.00", repo.credited)
	}
	if len(repo.settlements) != 1 ||
		repo.settlements[0].Status != "WON" ||
		repo.settlements[0].Payout.MinorUnits() != 10000 {
		t.Fatalf("settlements = %#v", repo.settlements)
	}
	if len(repo.events) != 1 || repo.events[0].CreatedAt != now {
		t.Fatalf("audit events = %#v", repo.events)
	}
	if err := service.SetResult(ctx, "M", &ResultInput{Outcome: "draw"}); err != nil {
		t.Fatal(err)
	}
	if repo.resultWrites != 1 || len(repo.events) != 1 || len(repo.settlements) != 1 {
		t.Fatal("repeated result wrote another transition")
	}
	if err := service.SetResult(ctx, "M", &ResultInput{
		Outcome:  "winner",
		WinnerID: &a,
	}); !errors.Is(err, ErrResultConflict) {
		t.Fatalf("conflicting result error = %v", err)
	}
}

type contextKey struct{}

func TestSetResultRecordsTeamCoinsOnceForWinningVotes(t *testing.T) {
	a, b := "A", "B"
	repo := &settlementRepositoryFake{
		match: Snapshot{ID: "M", TeamAID: &a, TeamBID: &b},
		tallies: []teamcoin.Tally{
			{ColorID: "GREEN", VoteRight: 40, VoteWrong: 35, BetsRight: 40, BetsWrong: 35},
			{ColorID: "PINK", VoteRight: 5, VoteWrong: 5, BetsRight: 5, BetsWrong: 5},
		},
	}
	now := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)
	ids := 0
	clock := func() time.Time { return now }
	newID := func() string {
		ids++

		return fmt.Sprintf("id-%d", ids)
	}
	service := NewService(nil, &settlementManagerFake{transaction: repo, team: repo}, clock, newID,
		teamcoin.NewService(value.MustMoneyFromMinor(100_00), clock, newID))

	if err := service.SetResult(context.Background(), "M", &ResultInput{Outcome: "winner", WinnerID: &a}); err != nil {
		t.Fatal(err)
	}
	if len(repo.teamCoinEvents) != 2 {
		t.Fatalf("team coin events = %#v; want one per color", repo.teamCoinEvents)
	}
	green, pink := repo.teamCoinEvents[0], repo.teamCoinEvents[1]
	if green.ColorID != "GREEN" || green.AmountDelta != 100_00 || green.Kind != teamcoin.KindSettled || green.MatchID != "M" || green.CreatedAt != now {
		t.Fatalf("green event = %#v", green)
	}
	if pink.ColorID != "PINK" || pink.AmountDelta != 0 || pink.BetsRightDelta != 5 || pink.BetsWrongDelta != 5 {
		t.Fatalf("pink event = %#v; a tie earns nothing but its bets still count", pink)
	}

	if err := service.SetResult(context.Background(), "M", &ResultInput{Outcome: "winner", WinnerID: &a}); err != nil {
		t.Fatal(err)
	}
	if len(repo.teamCoinEvents) != 2 {
		t.Fatal("repeated result wrote more team coin events")
	}
}
