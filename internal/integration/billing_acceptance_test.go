//go:build integration

package integration_test

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"os"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/esc-chula/intania-888-backend/internal/domain/bill"
	"github.com/esc-chula/intania-888-backend/internal/domain/match"
	"github.com/esc-chula/intania-888-backend/internal/domain/teamcoin"
	"github.com/esc-chula/intania-888-backend/internal/testutil"
	"github.com/esc-chula/intania-888-backend/internal/value"
)

type billingSuite struct {
	postgres *testutil.Postgres
	bills    *bill.Service
	matches  *match.Service
}

func newBillingSuite(t *testing.T) *billingSuite {
	t.Helper()

	postgres, err := testutil.OpenPostgres(os.Getenv("INTANIA888_TEST_DATABASE_URL"))
	if err != nil {
		if errors.Is(err, testutil.ErrMissingTestDatabaseURL) {
			t.Skip(err)
		}

		t.Fatal(err)
	}

	t.Cleanup(func() {
		if err := postgres.Close(); err != nil {
			t.Errorf("close integration database: %v", err)
		}
	})

	// The fixed per-match team coin award used by the integration suites.
	teamCoinsSvc := teamcoin.NewService(value.MustMoneyFromMinor(100_00), time.Now, uuid.NewString)

	billRepo := bill.NewGORMRepository(postgres.DB)
	matchRepo := match.NewGORMRepository(postgres.DB)

	return &billingSuite{
		postgres: postgres,
		bills:    bill.NewService(billRepo, billRepo, time.Now, uuid.NewString, teamCoinsSvc),
		matches:  match.NewService(matchRepo, matchRepo, time.Now, uuid.NewString, teamCoinsSvc),
	}
}

func (s *billingSuite) reset(t *testing.T) {
	t.Helper()

	if err := s.postgres.ResetAndMigrate(); err != nil {
		t.Fatal(err)
	}

	statements := []string{
		`INSERT INTO colors(id,title) VALUES('A','A'),('B','B'),('C','C')`,
		`INSERT INTO sport_types(id,title) VALUES('S','Test sport')`,
		`INSERT INTO locations(id,title) VALUES('TEST_LOCATION','Test location')`,
	}

	for _, statement := range statements {
		if _, err := s.postgres.SQL.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := s.postgres.SQL.Exec(
		`INSERT INTO users(id,email,name,role_id,remaining_coin) VALUES
			('U1','u1@example.test','User One','USER',$1),
			('U2','u2@example.test','User Two','USER',$2),
			('ADMIN','admin@example.test','Admin','ADMIN',$3)`,
		100_00,
		100_00,
		100_00,
	); err != nil {
		t.Fatal(err)
	}

	if _, err := s.postgres.SQL.Exec(
		`INSERT INTO matches(id,teama_id,teamb_id,type_id,location_id,start_time,end_time) VALUES
			('M1','A','B','S','TEST_LOCATION',now() + interval '1 hour',now() + interval '2 hours'),
			('M2','A','B','S','TEST_LOCATION',now() + interval '3 hours',now() + interval '4 hours'),
			('M3','A','B','S','TEST_LOCATION',now() + interval '5 hours',now() + interval '6 hours')`,
	); err != nil {
		t.Fatal(err)
	}
}

func money(minor int64) value.Money {
	return value.MustMoneyFromMinor(minor)
}

func line(matchID, teamID string) bill.Selection {
	return bill.Selection{
		MatchID:   matchID,
		BettingOn: teamID,
	}
}

func winner(id string) *string {
	return &id
}

func (s *billingSuite) place(t *testing.T, userID string, stake int64, lines ...bill.Selection) *bill.Result {
	t.Helper()

	created, err := s.bills.CreateBill(context.Background(), userID, &bill.CreateInput{
		Total: money(stake),
		Lines: lines,
	})
	if err != nil {
		t.Fatalf("place bill: %v", err)
	}

	return created
}

func (s *billingSuite) balance(t *testing.T, userID string) int64 {
	t.Helper()

	var balance int64
	if err := s.postgres.SQL.QueryRow(`SELECT remaining_coin FROM users WHERE id = $1`, userID).Scan(&balance); err != nil {
		t.Fatal(err)
	}

	return balance
}

func (s *billingSuite) billState(t *testing.T, billID string) (string, sql.NullInt64, sql.NullTime, sql.NullTime) {
	t.Helper()

	var status string
	var payout sql.NullInt64
	var settledAt sql.NullTime
	var voidedAt sql.NullTime

	err := s.postgres.SQL.QueryRow(`
SELECT status, payout, settled_at, voided_at
FROM bill_heads
WHERE id = $1`, billID).Scan(&status, &payout, &settledAt, &voidedAt)
	if err != nil {
		t.Fatal(err)
	}

	return status, payout, settledAt, voidedAt
}

func (s *billingSuite) eventCount(t *testing.T, billID string) int {
	t.Helper()

	var count int
	if err := s.postgres.SQL.QueryRow(`SELECT count(*) FROM bill_terminal_events WHERE bill_id = $1`, billID).Scan(&count); err != nil {
		t.Fatal(err)
	}

	return count
}

func waitForGroup(t *testing.T, group *sync.WaitGroup) {
	t.Helper()

	done := make(chan struct{})
	go func() {
		group.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("concurrent operation did not finish within timeout")
	}
}

func TestBillPlacementSnapshotsOddsAndDeductsAtomically(t *testing.T) {
	s := newBillingSuite(t)
	s.reset(t)

	created := s.place(t, "U1", 10_00, line("M1", "A"))

	if created.Status != "PENDING" {
		t.Fatalf("status = %s; want PENDING", created.Status)
	}

	if created.Lines[0].Rate.MicroUnits() != 2_000_000 {
		t.Fatalf("rate = %d; want 2000000", created.Lines[0].Rate.MicroUnits())
	}

	if got := s.balance(t, "U1"); got != 90_00 {
		t.Fatalf("balance = %d; want 90_00", got)
	}
}

func TestBillPlacementRejectsInvalidRequests(t *testing.T) {
	tests := []struct {
		name      string
		prepare   func(*testing.T, *billingSuite)
		request   bill.CreateInput
		wantError error
		wantCash  int64
	}{
		{
			name: "empty lines",
			request: bill.CreateInput{
				Total: money(10_00),
			},
			wantError: bill.ErrInvalidBill,
			wantCash:  100_00,
		},
		{
			name: "duplicate matches",
			request: bill.CreateInput{
				Total: money(10_00),
				Lines: []bill.Selection{line("M1", "A"), line("M1", "B")},
			},
			wantError: bill.ErrInvalidBill,
			wantCash:  100_00,
		},
		{
			name: "missing match",
			request: bill.CreateInput{
				Total: money(10_00),
				Lines: []bill.Selection{line("MISSING", "A")},
			},
			wantError: bill.ErrMatchNotFound,
			wantCash:  100_00,
		},
		{
			name: "invalid team",
			request: bill.CreateInput{
				Total: money(10_00),
				Lines: []bill.Selection{line("M1", "C")},
			},
			wantError: bill.ErrInvalidBill,
			wantCash:  100_00,
		},
		{
			name: "started match",
			prepare: func(t *testing.T, s *billingSuite) {
				t.Helper()

				if _, err := s.postgres.SQL.Exec(`UPDATE matches SET start_time = now() - interval '1 minute' WHERE id = 'M1'`); err != nil {
					t.Fatal(err)
				}
			},
			request: bill.CreateInput{
				Total: money(10_00),
				Lines: []bill.Selection{line("M1", "A")},
			},
			wantError: bill.ErrInvalidBill,
			wantCash:  100_00,
		},
		{
			name: "insufficient balance",
			prepare: func(t *testing.T, s *billingSuite) {
				t.Helper()

				if _, err := s.postgres.SQL.Exec(`UPDATE users SET remaining_coin = $1 WHERE id = 'U1'`, 1_00); err != nil {
					t.Fatal(err)
				}
			},
			request: bill.CreateInput{
				Total: money(10_00),
				Lines: []bill.Selection{line("M1", "A")},
			},
			wantError: bill.ErrInsufficientBalance,
			wantCash:  1_00,
		},
	}
	s := newBillingSuite(t)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s.reset(t)

			if tc.prepare != nil {
				tc.prepare(t, s)
			}

			_, err := s.bills.CreateBill(context.Background(), "U1", &tc.request)
			if !errors.Is(err, tc.wantError) {
				t.Fatalf("error = %v; want %v", err, tc.wantError)
			}

			if got := s.balance(t, "U1"); got != tc.wantCash {
				t.Fatalf("balance = %d; want %d", got, tc.wantCash)
			}
		})
	}
}

func TestBillPlacementRejectsPayoutOverflow(t *testing.T) {
	s := newBillingSuite(t)
	s.reset(t)

	if _, err := s.postgres.SQL.Exec(`UPDATE users SET remaining_coin = $1 WHERE id = 'U1'`, math.MaxInt64); err != nil {
		t.Fatal(err)
	}

	_, err := s.bills.CreateBill(context.Background(), "U1", &bill.CreateInput{
		Total: money(math.MaxInt64),
		Lines: []bill.Selection{line("M1", "A")},
	})
	if err == nil {
		t.Fatal("overflowing payout was accepted")
	}

	var count int
	if err := s.postgres.SQL.QueryRow(`SELECT count(*) FROM bill_heads`).Scan(&count); err != nil {
		t.Fatal(err)
	}

	if count != 0 {
		t.Fatalf("bill count = %d; want 0", count)
	}

	if got := s.balance(t, "U1"); got != math.MaxInt64 {
		t.Fatalf("balance = %d; want %d", got, int64(math.MaxInt64))
	}
}

func TestBillPlacementRollsBackOnBalanceFailure(t *testing.T) {
	s := newBillingSuite(t)
	s.reset(t)

	cleanup, err := s.postgres.InstallFailureTrigger("users", "UPDATE")
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		if err := cleanup(); err != nil {
			t.Errorf("remove failure trigger: %v", err)
		}
	}()

	if _, err := s.bills.CreateBill(context.Background(), "U1", &bill.CreateInput{
		Total: money(10_00),
		Lines: []bill.Selection{line("M1", "A")},
	}); err == nil {
		t.Fatal("placement succeeded despite injected balance failure")
	}

	var count int
	if err := s.postgres.SQL.QueryRow(`SELECT count(*) FROM bill_heads`).Scan(&count); err != nil {
		t.Fatal(err)
	}

	if count != 0 {
		t.Fatalf("bill count = %d; want 0", count)
	}

	if got := s.balance(t, "U1"); got != 100_00 {
		t.Fatalf("balance = %d; want 100_00", got)
	}
}

func TestConcurrentBillPlacementSerializesBalance(t *testing.T) {
	s := newBillingSuite(t)
	s.reset(t)

	if _, err := s.postgres.SQL.Exec(`UPDATE users SET remaining_coin = $1 WHERE id = 'U1'`, 10_00); err != nil {
		t.Fatal(err)
	}

	start := make(chan struct{})
	results := make(chan error, 2)
	var group sync.WaitGroup

	for i := 0; i < 2; i++ {
		group.Add(1)

		go func() {
			defer group.Done()
			<-start

			_, err := s.bills.CreateBill(context.Background(), "U1", &bill.CreateInput{
				Total: money(7_00),
				Lines: []bill.Selection{line("M1", "A")},
			})
			results <- err
		}()
	}

	close(start)
	waitForGroup(t, &group)
	close(results)

	var successes int
	var insufficient int
	for err := range results {
		if err == nil {
			successes++
		}

		if errors.Is(err, bill.ErrInsufficientBalance) {
			insufficient++
		}
	}

	if successes != 1 || insufficient != 1 {
		t.Fatalf("successes = %d, insufficient = %d; want one of each", successes, insufficient)
	}

	if got := s.balance(t, "U1"); got != 3_00 {
		t.Fatalf("balance = %d; want 3_00", got)
	}
}

func TestConcurrentBillPlacementSerializesOdds(t *testing.T) {
	s := newBillingSuite(t)
	s.reset(t)

	start := make(chan struct{})
	results := make(chan *bill.Result, 2)
	errorsCh := make(chan error, 2)
	var group sync.WaitGroup

	for _, userID := range []string{"U1", "U2"} {
		userID := userID
		group.Add(1)

		go func() {
			defer group.Done()
			<-start

			created, err := s.bills.CreateBill(context.Background(), userID, &bill.CreateInput{
				Total: money(10_00),
				Lines: []bill.Selection{line("M1", "A")},
			})
			if err != nil {
				errorsCh <- err

				return
			}

			results <- created
		}()
	}

	close(start)
	waitForGroup(t, &group)
	close(results)
	close(errorsCh)

	for err := range errorsCh {
		t.Fatal(err)
	}

	rates := make([]int64, 0, 2)
	for created := range results {
		rates = append(rates, created.Lines[0].Rate.MicroUnits())
	}
	sort.Slice(rates, func(i, j int) bool {
		return rates[i] < rates[j]
	})
	want := []int64{1_500_000, 2_000_000}

	if len(rates) != len(want) || rates[0] != want[0] || rates[1] != want[1] {
		t.Fatalf("rates = %v; want %v", rates, want)
	}
}

func TestSetResultSettlesAccumulatorAndIsIdempotent(t *testing.T) {
	s := newBillingSuite(t)
	s.reset(t)

	created := s.place(t, "U1", 10_00, line("M1", "A"), line("M2", "A"))

	if err := s.matches.SetResult(context.Background(), "M1", &match.ResultInput{
		Outcome:  "winner",
		WinnerID: winner("A"),
	}); err != nil {
		t.Fatal(err)
	}

	status, payout, settledAt, voidedAt := s.billState(t, created.ID)
	if status != "PENDING" || payout.Valid || settledAt.Valid || voidedAt.Valid {
		t.Fatalf("after first leg: status=%s payout=%v settled=%v voided=%v", status, payout, settledAt, voidedAt)
	}

	if got := s.balance(t, "U1"); got != 90_00 {
		t.Fatalf("balance after first leg = %d; want 90_00", got)
	}

	if err := s.matches.SetResult(context.Background(), "M2", &match.ResultInput{
		Outcome:  "winner",
		WinnerID: winner("A"),
	}); err != nil {
		t.Fatal(err)
	}

	status, payout, settledAt, voidedAt = s.billState(t, created.ID)
	if status != "WON" || !payout.Valid || payout.Int64 != 40_00 || !settledAt.Valid || voidedAt.Valid {
		t.Fatalf("after settlement: status=%s payout=%v settled=%v voided=%v", status, payout, settledAt, voidedAt)
	}

	if got := s.balance(t, "U1"); got != 130_00 {
		t.Fatalf("balance after settlement = %d; want 130_00", got)
	}

	if got := s.eventCount(t, created.ID); got != 1 {
		t.Fatalf("event count = %d; want 1", got)
	}

	if err := s.matches.SetResult(context.Background(), "M2", &match.ResultInput{
		Outcome:  "winner",
		WinnerID: winner("A"),
	}); err != nil {
		t.Fatal(err)
	}

	if got := s.balance(t, "U1"); got != 130_00 {
		t.Fatalf("balance after retry = %d; want 130_00", got)
	}

	if got := s.eventCount(t, created.ID); got != 1 {
		t.Fatalf("event count after retry = %d; want 1", got)
	}

	if err := s.matches.SetResult(context.Background(), "M2", &match.ResultInput{Outcome: "draw"}); !errors.Is(err, match.ErrResultConflict) {
		t.Fatalf("conflicting result error = %v; want %v", err, match.ErrResultConflict)
	}
}

func TestSetResultHandlesDrawAndLoss(t *testing.T) {
	tests := []struct {
		name       string
		result     *match.ResultInput
		wantStatus string
		wantPayout int64
		wantCash   int64
	}{
		{
			name:       "draw",
			result:     &match.ResultInput{Outcome: "draw"},
			wantStatus: "WON",
			wantPayout: 10_00,
			wantCash:   100_00,
		},
		{
			name: "loss",
			result: &match.ResultInput{
				Outcome:  "winner",
				WinnerID: winner("B"),
			},
			wantStatus: "LOST",
			wantPayout: 0,
			wantCash:   90_00,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := newBillingSuite(t)
			s.reset(t)

			created := s.place(t, "U1", 10_00, line("M1", "A"))

			if err := s.matches.SetResult(context.Background(), "M1", tc.result); err != nil {
				t.Fatal(err)
			}

			status, payout, _, _ := s.billState(t, created.ID)
			if status != tc.wantStatus || !payout.Valid || payout.Int64 != tc.wantPayout {
				t.Fatalf("status=%s payout=%v; want %s/%d", status, payout, tc.wantStatus, tc.wantPayout)
			}

			if got := s.balance(t, "U1"); got != tc.wantCash {
				t.Fatalf("balance = %d; want %d", got, tc.wantCash)
			}
		})
	}
}

func TestConcurrentSetResultIsIdempotent(t *testing.T) {
	s := newBillingSuite(t)
	s.reset(t)

	created := s.place(t, "U1", 10_00, line("M1", "A"))
	start := make(chan struct{})
	results := make(chan error, 2)
	var group sync.WaitGroup

	for i := 0; i < 2; i++ {
		group.Add(1)

		go func() {
			defer group.Done()
			<-start

			results <- s.matches.SetResult(context.Background(), "M1", &match.ResultInput{
				Outcome:  "winner",
				WinnerID: winner("A"),
			})
		}()
	}

	close(start)
	waitForGroup(t, &group)
	close(results)

	for err := range results {
		if err != nil {
			t.Fatal(err)
		}
	}

	status, payout, _, _ := s.billState(t, created.ID)
	if status != "WON" || !payout.Valid || payout.Int64 != 20_00 {
		t.Fatalf("status=%s payout=%v; want WON/20_00", status, payout)
	}

	if got := s.balance(t, "U1"); got != 110_00 {
		t.Fatalf("balance = %d; want 110_00", got)
	}

	if got := s.eventCount(t, created.ID); got != 1 {
		t.Fatalf("event count = %d; want 1", got)
	}
}

func TestConcurrentAccumulatorResultsSettleOnce(t *testing.T) {
	s := newBillingSuite(t)
	s.reset(t)

	created := s.place(t, "U1", 10_00, line("M1", "A"), line("M2", "A"))
	start := make(chan struct{})
	results := make(chan error, 2)
	var group sync.WaitGroup

	for _, matchID := range []string{"M1", "M2"} {
		matchID := matchID
		group.Add(1)

		go func() {
			defer group.Done()
			<-start

			results <- s.matches.SetResult(context.Background(), matchID, &match.ResultInput{
				Outcome:  "winner",
				WinnerID: winner("A"),
			})
		}()
	}

	close(start)
	waitForGroup(t, &group)
	close(results)

	for err := range results {
		if err != nil {
			t.Fatal(err)
		}
	}

	status, payout, _, _ := s.billState(t, created.ID)
	if status != "WON" || !payout.Valid || payout.Int64 != 40_00 {
		t.Fatalf("status=%s payout=%v; want WON/40_00", status, payout)
	}

	if got := s.balance(t, "U1"); got != 130_00 {
		t.Fatalf("balance = %d; want 130_00", got)
	}

	if got := s.eventCount(t, created.ID); got != 1 {
		t.Fatalf("event count = %d; want 1", got)
	}
}

func TestConcurrentConflictingResultsHaveOneTerminalOutcome(t *testing.T) {
	s := newBillingSuite(t)
	s.reset(t)

	created := s.place(t, "U1", 10_00, line("M1", "A"))
	start := make(chan struct{})
	results := make(chan error, 2)
	var group sync.WaitGroup

	group.Add(2)
	go func() {
		defer group.Done()
		<-start

		results <- s.matches.SetResult(context.Background(), "M1", &match.ResultInput{
			Outcome:  "winner",
			WinnerID: winner("A"),
		})
	}()

	go func() {
		defer group.Done()
		<-start

		results <- s.matches.SetResult(context.Background(), "M1", &match.ResultInput{Outcome: "draw"})
	}()

	close(start)
	waitForGroup(t, &group)
	close(results)

	var successes int
	var conflicts int
	for err := range results {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, match.ErrResultConflict):
			conflicts++
		default:
			t.Fatalf("unexpected concurrent result error: %v", err)
		}
	}

	if successes != 1 || conflicts != 1 {
		t.Fatalf("successes=%d conflicts=%d; want one of each", successes, conflicts)
	}

	status, payout, _, _ := s.billState(t, created.ID)
	if status != "WON" || !payout.Valid || (payout.Int64 != 10_00 && payout.Int64 != 20_00) {
		t.Fatalf("status=%s payout=%v; want WON with draw or winner payout", status, payout)
	}

	if got := s.eventCount(t, created.ID); got != 1 {
		t.Fatalf("event count = %d; want 1", got)
	}
}

func TestSetResultRollsBackOnAuditFailure(t *testing.T) {
	s := newBillingSuite(t)
	s.reset(t)

	created := s.place(t, "U1", 10_00, line("M1", "A"))
	cleanup, err := s.postgres.InstallFailureTrigger("bill_terminal_events", "INSERT")
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		if err := cleanup(); err != nil {
			t.Errorf("remove failure trigger: %v", err)
		}
	}()

	if err := s.matches.SetResult(context.Background(), "M1", &match.ResultInput{
		Outcome:  "winner",
		WinnerID: winner("A"),
	}); err == nil {
		t.Fatal("settlement succeeded despite injected audit failure")
	}

	var winnerID sql.NullString
	var isDraw bool
	if err := s.postgres.SQL.QueryRow(`SELECT winner_id, is_draw FROM matches WHERE id = 'M1'`).Scan(&winnerID, &isDraw); err != nil {
		t.Fatal(err)
	}

	if winnerID.Valid || isDraw {
		t.Fatalf("match result persisted after rollback: winner=%v draw=%v", winnerID, isDraw)
	}

	status, payout, settledAt, voidedAt := s.billState(t, created.ID)
	if status != "PENDING" || payout.Valid || settledAt.Valid || voidedAt.Valid {
		t.Fatalf("bill changed after rollback: status=%s payout=%v settled=%v voided=%v", status, payout, settledAt, voidedAt)
	}

	if got := s.balance(t, "U1"); got != 90_00 {
		t.Fatalf("balance after rollback = %d; want 90_00", got)
	}

	if got := s.eventCount(t, created.ID); got != 0 {
		t.Fatalf("event count after rollback = %d; want 0", got)
	}
}

func TestVoidRefundsPendingBillAndIsIdempotent(t *testing.T) {
	s := newBillingSuite(t)
	s.reset(t)

	created := s.place(t, "U1", 10_00, line("M1", "A"))

	if _, err := s.bills.VoidBill(context.Background(), created.ID, "ADMIN", "operator correction"); err != nil {
		t.Fatal(err)
	}

	status, payout, settledAt, voidedAt := s.billState(t, created.ID)
	if status != "VOIDED" || !payout.Valid || payout.Int64 != 10_00 || settledAt.Valid || !voidedAt.Valid {
		t.Fatalf("voided bill: status=%s payout=%v settled=%v voided=%v", status, payout, settledAt, voidedAt)
	}

	if got := s.balance(t, "U1"); got != 100_00 {
		t.Fatalf("balance after void = %d; want 100_00", got)
	}

	if got := s.eventCount(t, created.ID); got != 1 {
		t.Fatalf("event count = %d; want 1", got)
	}

	if _, err := s.bills.VoidBill(context.Background(), created.ID, "ADMIN", "second request"); err != nil {
		t.Fatal(err)
	}

	if got := s.balance(t, "U1"); got != 100_00 {
		t.Fatalf("balance after void retry = %d; want 100_00", got)
	}

	if got := s.eventCount(t, created.ID); got != 1 {
		t.Fatalf("event count after retry = %d; want 1", got)
	}
}

func TestVoidAllowsPendingAccumulatorAfterResolvedLeg(t *testing.T) {
	s := newBillingSuite(t)
	s.reset(t)

	created := s.place(t, "U1", 10_00, line("M1", "A"), line("M2", "A"))

	if err := s.matches.SetResult(context.Background(), "M1", &match.ResultInput{
		Outcome:  "winner",
		WinnerID: winner("A"),
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := s.bills.VoidBill(context.Background(), created.ID, "ADMIN", "cancel accumulator"); err != nil {
		t.Fatal(err)
	}

	status, payout, _, voidedAt := s.billState(t, created.ID)
	if status != "VOIDED" || !payout.Valid || payout.Int64 != 10_00 || !voidedAt.Valid {
		t.Fatalf("status=%s payout=%v voided=%v", status, payout, voidedAt)
	}

	if got := s.balance(t, "U1"); got != 100_00 {
		t.Fatalf("balance = %d; want 100_00", got)
	}
}

func TestVoidRejectsSettledBill(t *testing.T) {
	s := newBillingSuite(t)
	s.reset(t)

	created := s.place(t, "U1", 10_00, line("M1", "A"))

	if err := s.matches.SetResult(context.Background(), "M1", &match.ResultInput{
		Outcome:  "winner",
		WinnerID: winner("A"),
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := s.bills.VoidBill(context.Background(), created.ID, "ADMIN", "too late"); !errors.Is(err, bill.ErrBillConflict) {
		t.Fatalf("void error = %v; want %v", err, bill.ErrBillConflict)
	}
}

func TestVoidRollsBackOnAuditFailure(t *testing.T) {
	s := newBillingSuite(t)
	s.reset(t)

	created := s.place(t, "U1", 10_00, line("M1", "A"))
	cleanup, err := s.postgres.InstallFailureTrigger("bill_terminal_events", "INSERT")
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		if err := cleanup(); err != nil {
			t.Errorf("remove failure trigger: %v", err)
		}
	}()

	if _, err := s.bills.VoidBill(context.Background(), created.ID, "ADMIN", "triggered failure"); err == nil {
		t.Fatal("void succeeded despite injected audit failure")
	}

	status, payout, settledAt, voidedAt := s.billState(t, created.ID)
	if status != "PENDING" || payout.Valid || settledAt.Valid || voidedAt.Valid {
		t.Fatalf("bill changed after rollback: status=%s payout=%v settled=%v voided=%v", status, payout, settledAt, voidedAt)
	}

	if got := s.balance(t, "U1"); got != 90_00 {
		t.Fatalf("balance after rollback = %d; want 90_00", got)
	}

	if got := s.eventCount(t, created.ID); got != 0 {
		t.Fatalf("event count after rollback = %d; want 0", got)
	}
}

func TestVoidAndResultRaceHasOneTerminalTransition(t *testing.T) {
	s := newBillingSuite(t)
	s.reset(t)

	created := s.place(t, "U1", 10_00, line("M1", "A"))
	start := make(chan struct{})
	results := make(chan error, 2)
	var group sync.WaitGroup

	group.Add(2)
	go func() {
		defer group.Done()
		<-start

		_, err := s.bills.VoidBill(context.Background(), created.ID, "ADMIN", "race void")
		results <- err
	}()

	go func() {
		defer group.Done()
		<-start

		results <- s.matches.SetResult(context.Background(), "M1", &match.ResultInput{
			Outcome:  "winner",
			WinnerID: winner("A"),
		})
	}()

	close(start)
	waitForGroup(t, &group)
	close(results)

	for range results {
	}

	status, payout, settledAt, voidedAt := s.billState(t, created.ID)
	if status != "WON" && status != "VOIDED" {
		t.Fatalf("status = %s; want WON or VOIDED", status)
	}

	if status == "WON" && (!payout.Valid || payout.Int64 != 20_00 || !settledAt.Valid || voidedAt.Valid) {
		t.Fatalf("invalid won state: payout=%v settled=%v voided=%v", payout, settledAt, voidedAt)
	}

	if status == "VOIDED" && (!payout.Valid || payout.Int64 != 10_00 || settledAt.Valid || !voidedAt.Valid) {
		t.Fatalf("invalid voided state: payout=%v settled=%v voided=%v", payout, settledAt, voidedAt)
	}

	if got := s.eventCount(t, created.ID); got != 1 {
		t.Fatalf("event count = %d; want 1", got)
	}

	balance := s.balance(t, "U1")
	if balance != 100_00 && balance != 110_00 {
		t.Fatalf("balance = %d; want 100_00 or 110_00", balance)
	}
}

func TestSetResultRejectsWinnerOutsideMatch(t *testing.T) {
	s := newBillingSuite(t)
	s.reset(t)

	err := s.matches.SetResult(context.Background(), "M1", &match.ResultInput{
		Outcome:  "winner",
		WinnerID: winner("C"),
	})
	if !errors.Is(err, match.ErrInvalidResult) {
		t.Fatalf("error = %v; want %v", err, match.ErrInvalidResult)
	}
}

func TestConcurrentResultRequestsFinishWithinTimeout(t *testing.T) {
	s := newBillingSuite(t)
	s.reset(t)

	s.place(t, "U1", 10_00, line("M1", "A"))
	start := make(chan struct{})
	finished := make(chan struct{})

	go func() {
		<-start
		_ = s.matches.SetResult(context.Background(), "M1", &match.ResultInput{
			Outcome:  "winner",
			WinnerID: winner("A"),
		})
		close(finished)
	}()

	close(start)

	select {
	case <-finished:
	case <-time.After(5 * time.Second):
		t.Fatal("settlement did not finish within timeout")
	}
}

func TestBillTransactionCallbackRollsBackBalanceOnError(t *testing.T) {
	suite := newBillingSuite(t)
	suite.reset(t)
	repository := bill.NewGORMRepository(suite.postgres.DB)
	cause := errors.New("reject callback")
	err := repository.WithinTransaction(context.Background(), func(tx bill.TransactionRepository, _ teamcoin.Repository) error {
		if _, err := tx.LockBalance(context.Background(), "U1"); err != nil {
			return err
		}
		if err := tx.UpdateBalance(context.Background(), "U1", money(1_00)); err != nil {
			return err
		}

		return cause
	})
	if !errors.Is(err, cause) {
		t.Fatalf("transaction error = %v; want callback cause", err)
	}
	var balance int64
	if err := suite.postgres.SQL.QueryRow("SELECT remaining_coin FROM users WHERE id = 'U1'").Scan(&balance); err != nil {
		t.Fatal(err)
	}
	if balance != 100_00 {
		t.Fatalf("balance = %d; transaction failed to roll back", balance)
	}
}

func TestMatchTransactionCallbackRollsBackResultAndBalanceOnError(t *testing.T) {
	suite := newBillingSuite(t)
	suite.reset(t)
	repository := match.NewGORMRepository(suite.postgres.DB)
	cause := errors.New("reject callback")
	err := repository.WithinTransaction(context.Background(), func(tx match.TransactionRepository, _ teamcoin.Repository) error {
		if err := tx.AcquireLifecycleLock(context.Background()); err != nil {
			return err
		}
		matches, err := tx.LockMatches(context.Background(), []string{"M1"})
		if err != nil {
			return err
		}
		matches[0].IsDraw = true
		if err := tx.UpdateResult(context.Background(), &matches[0], time.Now()); err != nil {
			return err
		}
		if _, err := tx.LockUsers(context.Background(), []string{"U1"}); err != nil {
			return err
		}
		if err := tx.UpdateBalance(context.Background(), "U1", money(1_00)); err != nil {
			return err
		}

		return cause
	})
	if !errors.Is(err, cause) {
		t.Fatalf("transaction error = %v; want callback cause", err)
	}
	var draw bool
	if err := suite.postgres.SQL.QueryRow("SELECT is_draw FROM matches WHERE id = 'M1'").Scan(&draw); err != nil {
		t.Fatal(err)
	}
	var balance int64
	if err := suite.postgres.SQL.QueryRow("SELECT remaining_coin FROM users WHERE id = 'U1'").Scan(&balance); err != nil {
		t.Fatal(err)
	}
	if draw || balance != 100_00 {
		t.Fatalf("draw/balance = %v/%d; transaction failed to roll back", draw, balance)
	}
}
