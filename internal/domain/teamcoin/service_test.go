package teamcoin

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/esc-chula/intania-888-backend/internal/value"
)

const award = 100_00

func TestPlanAwardsOnlyWhenRightBeatsWrong(t *testing.T) {
	tests := []struct {
		name         string
		right, wrong int64
		want         int64
	}{
		{"worked example: 40 right vs 35 wrong", 40, 35, award},
		{"large margin pays the same", 60, 5, award},
		{"single bettor, right", 1, 0, award},
		{"exact tie earns nothing", 3, 3, 0},
		{"more wrong than right earns nothing", 2, 9, 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tally := Tally{ColorID: "GREEN", VoteRight: test.right, VoteWrong: test.wrong, BetsRight: test.right, BetsWrong: test.wrong}
			events := Plan([]Tally{tally}, nil, award)
			if len(events) != 1 || events[0].AmountDelta != test.want {
				t.Fatalf("events = %#v; want one event with amount %d", events, test.want)
			}
		})
	}
}

func TestPlanRecordsBetsEvenWhenNothingIsAwarded(t *testing.T) {
	events := Plan([]Tally{{ColorID: "PINK", VoteRight: 1, VoteWrong: 2, BetsRight: 1, BetsWrong: 2}}, nil, award)
	want := []Event{{ColorID: "PINK", AmountDelta: 0, BetsRightDelta: 1, BetsWrongDelta: 2, VoteRight: 1, VoteWrong: 2}}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %#v; want %#v", events, want)
	}
}

func TestPlanWritesOnlyTheDifference(t *testing.T) {
	// The ledger already holds the win; a voided bet flips the vote to a loss.
	tally := Tally{ColorID: "GREEN", VoteRight: 34, VoteWrong: 35, BetsRight: 39, BetsWrong: 35}
	balance := Balance{ColorID: "GREEN", Amount: award, BetsRight: 40, BetsWrong: 35}
	events := Plan([]Tally{tally}, []Balance{balance}, award)
	if len(events) != 1 || events[0].AmountDelta != -award || events[0].BetsRightDelta != -1 || events[0].BetsWrongDelta != 0 {
		t.Fatalf("events = %#v; want the award reversed and one right bet removed", events)
	}

	// A void that changes neither the vote result nor the counts records nothing.
	if events := Plan([]Tally{{ColorID: "GREEN", VoteRight: 40, VoteWrong: 35, BetsRight: 40, BetsWrong: 35}},
		[]Balance{{ColorID: "GREEN", Amount: award, BetsRight: 40, BetsWrong: 35}}, award); len(events) != 0 {
		t.Fatalf("events = %#v; want none when the ledger is already correct", events)
	}
}

func TestPlanZeroesColorsWhoseBetsWereAllVoided(t *testing.T) {
	events := Plan(nil, []Balance{{ColorID: "BLUE", Amount: award, BetsRight: 1}}, award)
	if len(events) != 1 || events[0].ColorID != "BLUE" || events[0].AmountDelta != -award || events[0].BetsRightDelta != -1 {
		t.Fatalf("events = %#v", events)
	}
}

func TestPlanOrdersEventsByColor(t *testing.T) {
	events := Plan([]Tally{
		{ColorID: "PINK", VoteRight: 1, BetsRight: 1},
		{ColorID: "BLUE", VoteRight: 1, BetsRight: 1},
	}, nil, award)
	if len(events) != 2 || events[0].ColorID != "BLUE" || events[1].ColorID != "PINK" {
		t.Fatalf("events = %#v", events)
	}
}

type fakeRepository struct {
	tallies  []Tally
	balances []Balance
	created  []Event
	err      error
}

func (s *fakeRepository) TeamVoteTallies(context.Context, string) ([]Tally, error) {
	return s.tallies, s.err
}

func (s *fakeRepository) TeamCoinBalances(context.Context, string) ([]Balance, error) {
	return s.balances, nil
}

func (s *fakeRepository) CreateTeamCoinEvents(_ context.Context, events []Event) error {
	s.created = append(s.created, events...)

	return nil
}

func counter() func() string {
	n := 0

	return func() string {
		n++

		return fmt.Sprintf("event-%d", n)
	}
}

func TestSettleAndAdjustStampEventsAndSkipWhenNothingChanged(t *testing.T) {
	now := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	store := &fakeRepository{tallies: []Tally{{ColorID: "GREEN", VoteRight: 2, VoteWrong: 1, BetsRight: 2, BetsWrong: 1}}}
	service := NewService(value.MustMoneyFromMinor(award), func() time.Time { return now }, counter())

	if err := service.Settle(context.Background(), store, "M"); err != nil {
		t.Fatal(err)
	}
	want := Event{ID: "event-1", MatchID: "M", ColorID: "GREEN", Kind: KindSettled, AmountDelta: award,
		BetsRightDelta: 2, BetsWrongDelta: 1, VoteRight: 2, VoteWrong: 1, CreatedAt: now}
	if len(store.created) != 1 || store.created[0] != want {
		t.Fatalf("created = %#v; want %#v", store.created, want)
	}

	// Once the ledger matches the bets, repeating records nothing.
	store.balances = []Balance{{ColorID: "GREEN", Amount: award, BetsRight: 2, BetsWrong: 1}}
	store.created = nil
	if err := service.Settle(context.Background(), store, "M"); err != nil || len(store.created) != 0 {
		t.Fatalf("repeat settle created %#v, err %v; want nothing", store.created, err)
	}

	// A void that flips the vote records an adjustment tied to the bill.
	store.tallies = []Tally{{ColorID: "GREEN", VoteRight: 1, VoteWrong: 1, BetsRight: 1, BetsWrong: 1}}
	if err := service.Adjust(context.Background(), store, "M", "bill"); err != nil {
		t.Fatal(err)
	}
	if len(store.created) != 1 || store.created[0].Kind != KindAdjusted || store.created[0].BillID != "bill" ||
		store.created[0].AmountDelta != -award || store.created[0].BetsRightDelta != -1 {
		t.Fatalf("adjustment = %#v", store.created)
	}
}

func TestServiceRejectsInvalidInputAndPropagatesStoreErrors(t *testing.T) {
	now := func() time.Time { return time.Time{} }
	good := NewService(value.MustMoneyFromMinor(award), now, counter())

	if err := good.Settle(context.Background(), &fakeRepository{}, ""); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("empty match error = %v; want ErrInvalidInput", err)
	}
	if err := good.Adjust(context.Background(), &fakeRepository{}, "M", ""); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("adjust without a bill error = %v; want ErrInvalidInput", err)
	}
	for _, bad := range []*Service{
		NewService(value.MustMoneyFromMinor(award), nil, counter()),
		NewService(value.MustMoneyFromMinor(award), now, nil),
	} {
		if err := bad.Settle(context.Background(), &fakeRepository{}, "M"); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("missing clock or ID source error = %v; want ErrInvalidInput", err)
		}
	}

	cause := errors.New("database down")
	if err := good.Settle(context.Background(), &fakeRepository{err: cause}, "M"); !errors.Is(err, cause) {
		t.Fatalf("error = %v; want the store error", err)
	}
}
