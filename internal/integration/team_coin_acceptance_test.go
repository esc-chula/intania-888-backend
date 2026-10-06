//go:build integration

package integration_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/esc-chula/intania-888-backend/internal/domain/color"
	"github.com/esc-chula/intania-888-backend/internal/domain/match"
)

// addTeamUsers creates n USER accounts in a group of the given color and returns their IDs.
func (s *billingSuite) addTeamUsers(t *testing.T, colorID string, n int) []string {
	t.Helper()

	if _, err := s.postgres.SQL.Exec(`INSERT INTO intania_groups(id,color_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, "G_"+colorID, colorID); err != nil {
		t.Fatal(err)
	}

	ids := make([]string, n)
	for i := range ids {
		ids[i] = fmt.Sprintf("%s%d", colorID, i+1)
		if _, err := s.postgres.SQL.Exec(
			`INSERT INTO users(id,email,name,role_id,group_id,remaining_coin) VALUES($1,$2,$1,'USER',$3,100_00)`,
			ids[i], ids[i]+"@example.test", "G_"+colorID,
		); err != nil {
			t.Fatal(err)
		}
	}

	return ids
}

func (s *billingSuite) setWinner(t *testing.T, matchID, teamID string) {
	t.Helper()

	if err := s.matches.SetResult(context.Background(), matchID, &match.ResultInput{Outcome: "winner", WinnerID: winner(teamID)}); err != nil {
		t.Fatalf("set result: %v", err)
	}
}

func (s *billingSuite) void(t *testing.T, billID string) {
	t.Helper()

	if _, err := s.bills.VoidBill(context.Background(), billID, "ADMIN", "test"); err != nil {
		t.Fatalf("void bill: %v", err)
	}
}

// ledger returns a color's summed ledger values for a match and its number of rows.
func (s *billingSuite) ledger(t *testing.T, matchID, colorID string) (amount, right, wrong int64, rows int) {
	t.Helper()

	err := s.postgres.SQL.QueryRow(`
SELECT COALESCE(SUM(amount_delta),0), COALESCE(SUM(bets_right_delta),0), COALESCE(SUM(bets_wrong_delta),0), COUNT(*)
FROM team_coin_events WHERE match_id=$1 AND color_id=$2`, matchID, colorID).Scan(&amount, &right, &wrong, &rows)
	if err != nil {
		t.Fatal(err)
	}

	return amount, right, wrong, rows
}

// wantTotalsMatchLedger checks that the totals kept on colors equal the sums of the ledger.
func (s *billingSuite) wantTotalsMatchLedger(t *testing.T) {
	t.Helper()

	rows, err := s.postgres.SQL.Query(`
SELECT c.id, c.team_coin, c.bets_right, c.bets_wrong,
	COALESCE(SUM(e.amount_delta),0), COALESCE(SUM(e.bets_right_delta),0), COALESCE(SUM(e.bets_wrong_delta),0)
FROM colors c LEFT JOIN team_coin_events e ON e.color_id = c.id
GROUP BY c.id`)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			t.Error(err)
		}
	}()
	for rows.Next() {
		var id string
		var coin, right, wrong, ledgerCoin, ledgerRight, ledgerWrong int64
		if err := rows.Scan(&id, &coin, &right, &wrong, &ledgerCoin, &ledgerRight, &ledgerWrong); err != nil {
			t.Fatal(err)
		}
		if coin != ledgerCoin || right != ledgerRight || wrong != ledgerWrong {
			t.Fatalf("color %s totals = %d/%d/%d; ledger sums = %d/%d/%d", id, coin, right, wrong, ledgerCoin, ledgerRight, ledgerWrong)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
}

func (s *billingSuite) ledgerRows(t *testing.T) int {
	t.Helper()

	var rows int
	if err := s.postgres.SQL.QueryRow(`SELECT COUNT(*) FROM team_coin_events`).Scan(&rows); err != nil {
		t.Fatal(err)
	}

	return rows
}

func (s *billingSuite) wantLedger(t *testing.T, matchID, colorID string, amount, right, wrong int64) {
	t.Helper()

	gotAmount, gotRight, gotWrong, _ := s.ledger(t, matchID, colorID)
	if gotAmount != amount || gotRight != right || gotWrong != wrong {
		t.Fatalf("ledger %s/%s = amount %d right %d wrong %d; want %d %d %d",
			matchID, colorID, gotAmount, gotRight, gotWrong, amount, right, wrong)
	}
}

func TestTeamCoinsAwardOnlyWhenRightBettorsOutnumberWrongOnes(t *testing.T) {
	s := newBillingSuite(t)
	s.reset(t)
	green := s.addTeamUsers(t, "C", 6) // the worked example, scaled down: 3 right vs 2 wrong, 1 does not bet
	for _, id := range green[:3] {
		s.place(t, id, 10_00, line("M1", "A"))
	}
	for _, id := range green[3:5] {
		s.place(t, id, 10_00, line("M1", "B"))
	}
	// A color's own members and accounts without a group or a USER role are handled too.
	s.place(t, "U1", 10_00, line("M1", "A")) // no group, so no color

	s.setWinner(t, "M1", "A")

	s.wantLedger(t, "M1", "C", 100_00, 3, 2)
	s.wantTotalsMatchLedger(t)
	if _, _, _, rows := s.ledger(t, "M1", "A"); rows != 0 {
		t.Fatalf("a color without bettors got %d ledger rows", rows)
	}
	if got := s.ledgerRows(t); got != 1 {
		t.Fatalf("ledger rows = %d; want only the green team's row (U1 has no color)", got)
	}
}

func TestTeamCoinsTieEarnsNothingButBetsStillCount(t *testing.T) {
	s := newBillingSuite(t)
	s.reset(t)
	green := s.addTeamUsers(t, "C", 4)
	s.place(t, green[0], 10_00, line("M1", "A"))
	s.place(t, green[1], 10_00, line("M1", "A"))
	s.place(t, green[2], 10_00, line("M1", "B"))
	s.place(t, green[3], 10_00, line("M1", "B"))

	s.setWinner(t, "M1", "A")

	s.wantLedger(t, "M1", "C", 0, 2, 2)
	s.wantTotalsMatchLedger(t)
}

func TestTeamCoinsAreRecordedOnceAndNotForDraws(t *testing.T) {
	s := newBillingSuite(t)
	s.reset(t)
	green := s.addTeamUsers(t, "C", 2)
	s.place(t, green[0], 10_00, line("M1", "A"))
	s.place(t, green[1], 10_00, line("M1", "A"))
	s.place(t, green[0], 10_00, line("M2", "A"))

	s.setWinner(t, "M1", "A")
	s.setWinner(t, "M1", "A") // an identical result is a no-op
	if got := s.ledgerRows(t); got != 1 {
		t.Fatalf("ledger rows after a repeated result = %d; want 1", got)
	}

	if err := s.matches.SetResult(context.Background(), "M2", &match.ResultInput{Outcome: "draw"}); err != nil {
		t.Fatal(err)
	}
	if _, _, _, rows := s.ledger(t, "M2", "C"); rows != 0 {
		t.Fatalf("a drawn match produced %d ledger rows", rows)
	}
}

func TestVoidReversesTeamCoinsWhenTheVoteFlips(t *testing.T) {
	s := newBillingSuite(t)
	s.reset(t)
	green := s.addTeamUsers(t, "C", 3)
	// Two right bettors and one wrong one win the vote. The first right bettor's bill also covers
	// M2, which is undecided, so it is still pending after M1 is decided and can be voided.
	voidable := s.place(t, green[0], 10_00, line("M1", "A"), line("M2", "A"))
	s.place(t, green[1], 10_00, line("M1", "A"))
	s.place(t, green[2], 10_00, line("M1", "B"))

	s.setWinner(t, "M1", "A")
	s.wantLedger(t, "M1", "C", 100_00, 2, 1)

	s.void(t, voidable.ID)

	s.wantLedger(t, "M1", "C", 0, 1, 1) // 1 right vs 1 wrong is a tie now
	s.wantTotalsMatchLedger(t)
	if _, _, _, rows := s.ledger(t, "M1", "C"); rows != 2 {
		t.Fatalf("ledger rows = %d; want the settled row plus one adjustment", rows)
	}
	var kind, billID string
	if err := s.postgres.SQL.QueryRow(`SELECT kind, bill_id FROM team_coin_events WHERE match_id='M1' AND color_id='C' AND kind='ADJUSTED'`).Scan(&kind, &billID); err != nil || billID != voidable.ID {
		t.Fatalf("adjustment bill = %q, err %v; want %s", billID, err, voidable.ID)
	}

	// Voiding again is a no-op for both the bill and the ledger.
	s.void(t, voidable.ID)
	if got := s.ledgerRows(t); got != 2 {
		t.Fatalf("ledger rows after a repeated void = %d; want 2", got)
	}

	standings, err := color.NewGORMRepository(s.postgres.DB).CoinStandings(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, standing := range standings {
		if standing.ID == "C" && (standing.TotalCoin.MinorUnits() != 0) {
			t.Fatalf("green standing = %+v; want 0 coins after the reversal", standing)
		}
	}
}

func TestVoidKeepsTeamCoinsWhenTheVoteStillWins(t *testing.T) {
	s := newBillingSuite(t)
	s.reset(t)
	green := s.addTeamUsers(t, "C", 4)
	voidable := s.place(t, green[0], 10_00, line("M1", "A"), line("M2", "A"))
	s.place(t, green[1], 10_00, line("M1", "A"))
	s.place(t, green[2], 10_00, line("M1", "A"))
	s.place(t, green[3], 10_00, line("M1", "B"))

	s.setWinner(t, "M1", "A")
	s.void(t, voidable.ID)

	s.wantLedger(t, "M1", "C", 100_00, 2, 1) // still won; only the voided right bet is removed
	s.wantTotalsMatchLedger(t)
}

func TestVoidOfBillOnUndecidedMatchesWritesNothing(t *testing.T) {
	s := newBillingSuite(t)
	s.reset(t)
	green := s.addTeamUsers(t, "C", 1)
	pending := s.place(t, green[0], 10_00, line("M1", "A"), line("M2", "B"))

	s.void(t, pending.ID)

	if got := s.ledgerRows(t); got != 0 {
		t.Fatalf("ledger rows = %d; want none", got)
	}

	s.setWinner(t, "M1", "A") // the voided bet no longer counts
	if got := s.ledgerRows(t); got != 0 {
		t.Fatalf("ledger rows after settling = %d; want none because the only bet was voided", got)
	}
}

func TestColorRankingsReadTheLedger(t *testing.T) {
	s := newBillingSuite(t)
	s.reset(t)
	green := s.addTeamUsers(t, "C", 2)
	blue := s.addTeamUsers(t, "B", 2)
	// Green: 2 right on M1 (win) and 1 wrong on M2 (loss). Blue: 1 right and 1 wrong on M1 (tie).
	s.place(t, green[0], 10_00, line("M1", "A"), line("M2", "B"))
	s.place(t, green[1], 10_00, line("M1", "A"))
	s.place(t, blue[0], 10_00, line("M1", "A"))
	s.place(t, blue[1], 10_00, line("M1", "B"))
	s.setWinner(t, "M1", "A")
	s.setWinner(t, "M2", "A")

	repo := color.NewGORMRepository(s.postgres.DB)
	coins, err := repo.CoinStandings(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	predictions, err := repo.PredictionStandings(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	s.wantTotalsMatchLedger(t)
	wantCoin := map[string]int64{"A": 0, "B": 0, "C": 100_00}
	for _, standing := range coins {
		if standing.TotalCoin.MinorUnits() != wantCoin[standing.ID] {
			t.Fatalf("coins[%s] = %d; want %d", standing.ID, standing.TotalCoin.MinorUnits(), wantCoin[standing.ID])
		}
	}
	wantBets := map[string][2]int64{"A": {0, 0}, "B": {1, 1}, "C": {2, 1}}
	for _, standing := range predictions {
		if got := [2]int64{standing.Correct, standing.Wrong}; got != wantBets[standing.ID] {
			t.Fatalf("bets[%s] = %v; want %v", standing.ID, got, wantBets[standing.ID])
		}
	}
}
