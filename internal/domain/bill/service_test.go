package bill

import (
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/esc-chula/intania-888-backend/internal/domain/match"
	"github.com/esc-chula/intania-888-backend/internal/domain/teamcoin"
	"github.com/esc-chula/intania-888-backend/internal/value"
)

func TestSeededRate(t *testing.T) {
	tests := []struct {
		a, b int64
		forA bool
		want int64
	}{
		{
			a:    0,
			b:    0,
			forA: true,
			want: 2000000,
		},
		{
			a:    0,
			b:    0,
			forA: false,
			want: 2000000,
		},
		{
			a:    3,
			b:    1,
			forA: true,
			want: 1500000,
		},
		{
			a:    3,
			b:    1,
			forA: false,
			want: 3000000,
		},
		{
			a:    1,
			b:    2,
			forA: true,
			want: 2500000,
		},
		{
			a:    math.MaxInt64,
			b:    math.MaxInt64,
			forA: true,
			want: 2000000,
		},
	}

	for _, tc := range tests {
		got, err := seededRate(tc.a, tc.b, tc.forA)

		if err != nil || got.MicroUnits() != tc.want {
			t.Fatalf("seededRate(%d,%d,%v)=%d,%v; want %d", tc.a, tc.b, tc.forA, got.MicroUnits(), err, tc.want)
		}
	}
}

func TestSeededRateRejectsInvalidAndOverflowingCounts(t *testing.T) {
	if _, err := seededRate(-1, 0, true); err == nil {
		t.Fatal("negative count must be rejected")
	}

	if _, err := seededRate(math.MaxInt64, 0, false); err == nil {
		t.Fatal("unrepresentable rate must be rejected")
	}
}

type transactionManagerFake struct {
	transaction TransactionRepository
	context     context.Context
	commitErr   error
	calls       int
}

func (f *transactionManagerFake) WithinTransaction(ctx context.Context, callback func(TransactionRepository, teamcoin.Repository) error) error {
	f.context = ctx
	f.calls++
	if err := callback(f.transaction, nil); err != nil {
		return err
	}

	return f.commitErr
}

type billTransactionFake struct {
	TransactionRepository
	steps    []string
	now      time.Time
	balance  value.Money
	debitErr error
	created  *Result
}

func (f *billTransactionFake) AcquireLifecycleLock(context.Context) error {
	f.steps = append(f.steps, "guard")

	return nil
}

func (f *billTransactionFake) LockMatches(_ context.Context, ids []string) ([]match.Snapshot, error) {
	f.steps = append(f.steps, "matches")
	a, b := "A", "B"
	rows := make([]match.Snapshot, len(ids))
	for i, id := range ids {
		rows[i] = match.Snapshot{
			ID:        id,
			TeamAID:   &a,
			TeamBID:   &b,
			StartTime: f.now.Add(time.Hour),
		}
	}

	return rows, nil
}

func (f *billTransactionFake) CountBets(context.Context, string) ([]BetCount, error) {
	f.steps = append(f.steps, "counts")

	return []BetCount{{
		BettingOn: "A",
		Count:     3,
	}, {
		BettingOn: "B",
		Count:     1,
	}}, nil
}

func (f *billTransactionFake) LockBalance(context.Context, string) (value.Money, error) {
	f.steps = append(f.steps, "user")

	return f.balance, nil
}

func (f *billTransactionFake) LockVoteColor(context.Context, string) (*string, error) {
	f.steps = append(f.steps, "vote-color")

	return nil, nil
}

func (f *billTransactionFake) CreateBill(_ context.Context, bill *Result) error {
	f.steps = append(f.steps, "create")
	f.created = bill

	return nil
}

func (f *billTransactionFake) DebitBalance(context.Context, string, value.Money) error {
	f.steps = append(f.steps, "debit")

	return f.debitErr
}

func TestCreateBillUsesAuthoritativeRatesAndTransactionOrder(t *testing.T) {
	now := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)
	tx := &billTransactionFake{
		now:     now,
		balance: value.MustMoneyFromMinor(20000),
	}
	manager := &transactionManagerFake{transaction: tx}
	service := NewService(nil, manager, func() time.Time {
		return now
	}, func() string {
		return "bill"
	}, teamCoinsSvc())
	ctx := context.WithValue(context.Background(), contextKey{}, "request")
	input := &CreateInput{
		Total: value.MustMoneyFromMinor(10000),
		Lines: []Selection{{
			MatchID:   "M2",
			BettingOn: "B",
		}, {
			MatchID:   "M1",
			BettingOn: "A",
		}},
	}
	result, err := service.CreateBill(ctx, "user", input)
	if err != nil {
		t.Fatal(err)
	}
	if manager.context != ctx {
		t.Fatal("request context was not propagated")
	}
	if result.Lines[0].MatchID != "M1" ||
		result.Lines[0].Rate.MicroUnits() != 1500000 ||
		result.Lines[1].Rate.MicroUnits() != 3000000 {
		t.Fatalf("authoritative selections = %#v", result.Lines)
	}
	if input.Lines[0].MatchID != "M2" {
		t.Fatal("caller selections were reordered")
	}
	want := []string{"guard", "matches", "counts", "counts", "user", "vote-color", "create", "debit"}
	if !reflect.DeepEqual(tx.steps, want) {
		t.Fatalf("transaction steps = %v; want %v", tx.steps, want)
	}
}

// teamCoinsSvc returns the team coin service the services under test are built with; these
// tests never void a bill, so it is never called.
func teamCoinsSvc() TeamCoins {
	return teamcoin.NewService(value.MustMoneyFromMinor(100_00), time.Now, func() string { return "event" })
}

type contextKey struct{}

func TestCreateBillReturnsNoResultWhenTransactionFails(t *testing.T) {
	for _, commitFailure := range []bool{false, true} {
		t.Run(fmt.Sprint(commitFailure), func(t *testing.T) {
			cause := errors.New("transaction failure")
			now := time.Now()
			tx := &billTransactionFake{
				now:     now,
				balance: value.MustMoneyFromMinor(20000),
			}
			manager := &transactionManagerFake{transaction: tx}
			if commitFailure {
				manager.commitErr = cause
			} else {
				tx.debitErr = cause
			}
			service := NewService(nil, manager, func() time.Time {
				return now
			}, func() string {
				return "bill"
			}, teamCoinsSvc())
			result, err := service.CreateBill(context.Background(), "user", &CreateInput{
				Total: value.MustMoneyFromMinor(10000),
				Lines: []Selection{{
					MatchID:   "M",
					BettingOn: "A",
				}},
			})
			if result != nil || !errors.Is(err, cause) {
				t.Fatalf("result/error = %v/%v; want nil/original cause", result, err)
			}
		})
	}
}

func TestCreateBillRejectsDuplicateMatchesBeforeTransaction(t *testing.T) {
	manager := &transactionManagerFake{}
	service := NewService(nil, manager, time.Now, func() string {
		return "bill"
	}, teamCoinsSvc())
	_, err := service.CreateBill(context.Background(), "user", &CreateInput{
		Total: value.MustMoneyFromMinor(10000),
		Lines: []Selection{{
			MatchID:   "M",
			BettingOn: "A",
		}, {
			MatchID:   "M",
			BettingOn: "B",
		}},
	})
	if !errors.Is(err, ErrInvalidBill) || manager.calls != 0 {
		t.Fatalf("error/calls = %v/%d", err, manager.calls)
	}
}
