//go:build integration

package integration_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"sync"
	"testing"

	"go.uber.org/zap"

	"github.com/esc-chula/intania-888-backend/internal/domain/stakemine"
	persistence "github.com/esc-chula/intania-888-backend/internal/persistence/model"
	"github.com/esc-chula/intania-888-backend/internal/testutil"
	"github.com/esc-chula/intania-888-backend/internal/value"
)

func openStakeMinePostgres(t *testing.T) *testutil.Postgres {
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

	if err := postgres.ResetAndMigrate(); err != nil {
		t.Fatal(err)
	}

	return postgres
}

func stakeMineGrid(t *testing.T, revealed int) string {
	t.Helper()

	type tile struct {
		Index    int    `json:"index"`
		Type     string `json:"type"`
		Revealed bool   `json:"revealed"`
	}
	grid := make([]tile, 16)
	for i := range grid {
		grid[i] = tile{
			Index:    i,
			Type:     "diamond",
			Revealed: i < revealed,
		}
	}

	grid[14].Type = "bomb"
	grid[15].Type = "bomb"

	gridJSON, err := json.Marshal(grid)
	if err != nil {
		t.Fatal(err)
	}

	return string(gridJSON)
}

func seedStakeMineUser(t *testing.T, postgres *testutil.Postgres, userID string, balance int64) {
	t.Helper()

	_, err := postgres.SQL.Exec(
		`INSERT INTO users(id, email, name, role_id, remaining_coin)
		 VALUES($1, $2, $3, 'USER', $4)`,
		userID,
		userID+"@example.test",
		userID,
		balance,
	)
	if err != nil {
		t.Fatal(err)
	}
}

func seedStakeMineGame(t *testing.T, postgres *testutil.Postgres, gameID, userID string, revealedCount int, payout int64) {
	t.Helper()

	_, err := postgres.SQL.Exec(
		`INSERT INTO mine_games
			 (id, user_id, bet_amount, risk_level, status, revealed_count, current_payout, multiplier, grid_data)
		 VALUES($1, $2, $3, 'low', 'active', $4, $5, 1000000, $6)`,
		gameID,
		userID,
		100_00,
		revealedCount,
		payout,
		stakeMineGrid(t, revealedCount),
	)
	if err != nil {
		t.Fatal(err)
	}
}

func TestStakeMineConcurrentCashOutPaysOnce(t *testing.T) {
	postgres := openStakeMinePostgres(t)
	seedStakeMineUser(t, postgres, "cashout-user", 500_00)
	seedStakeMineGame(t, postgres, "cashout-game", "cashout-user", 1, 250_00)

	service := stakemine.NewService(
		stakemine.NewGORMRepository(postgres.DB),
		zap.NewNop(),
	)

	start := make(chan struct{})
	errs := make(chan error, 2)
	var wg sync.WaitGroup

	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := service.CashOut(context.Background(), "cashout-user", "cashout-game")
			errs <- err
		}()
	}

	close(start)
	wg.Wait()
	close(errs)

	successes := 0
	for err := range errs {
		if err == nil {
			successes++
		}
	}

	if successes != 1 {
		t.Fatalf("concurrent cash-out successes = %d, want 1", successes)
	}

	var user persistence.User
	if err := postgres.DB.First(&user, "id = ?", "cashout-user").Error; err != nil {
		t.Fatal(err)
	}

	if user.RemainingCoin != 750_00 {
		t.Fatalf("user balance = %d, want 750_00", user.RemainingCoin)
	}

	var game persistence.MineGame
	if err := postgres.DB.First(&game, "id = ?", "cashout-game").Error; err != nil {
		t.Fatal(err)
	}

	if game.Status != "cashed_out" {
		t.Fatalf("game status = %q, want cashed_out", game.Status)
	}
}

func TestStakeMineHistoryFailureRollsBackTerminalPayout(t *testing.T) {
	postgres := openStakeMinePostgres(t)
	seedStakeMineUser(t, postgres, "history-user", 500_00)
	seedStakeMineGame(t, postgres, "history-game", "history-user", 13, 250_00)

	cleanup, err := postgres.InstallFailureTrigger("mine_game_histories", "INSERT")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := cleanup(); err != nil {
			t.Errorf("remove failure trigger: %v", err)
		}
	}()

	service := stakemine.NewService(
		stakemine.NewGORMRepository(postgres.DB),
		zap.NewNop(),
	)

	_, _, err = service.RevealTile(context.Background(), "history-user", "history-game", stakemine.RevealInput{Index: 13})
	if err == nil {
		t.Fatal("terminal reveal succeeded despite history failure")
	}

	var user persistence.User
	if err := postgres.DB.First(&user, "id = ?", "history-user").Error; err != nil {
		t.Fatal(err)
	}

	if user.RemainingCoin != 500_00 {
		t.Fatalf("user balance = %d, want 500_00", user.RemainingCoin)
	}

	var game persistence.MineGame
	if err := postgres.DB.First(&game, "id = ?", "history-game").Error; err != nil {
		t.Fatal(err)
	}

	if game.Status != "active" {
		t.Fatalf("game status = %q, want active", game.Status)
	}
}

func TestStakeMineConcurrentCreationFundsOneActiveGame(t *testing.T) {
	postgres := openStakeMinePostgres(t)
	seedStakeMineUser(t, postgres, "create-user", 500_00)

	service := stakemine.NewService(stakemine.NewGORMRepository(postgres.DB), zap.NewNop())
	start := make(chan struct{})
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := service.CreateGame(context.Background(), "create-user", stakemine.CreateInput{
				BetAmount: value.MustMoneyFromMinor(100_00),
				RiskLevel: "low",
			})
			errs <- err
		}()
	}
	close(start)
	wg.Wait()
	close(errs)

	successes := 0
	for err := range errs {
		if err == nil {
			successes++
		} else if !errors.Is(err, stakemine.ErrGameConflict) {
			t.Fatalf("unexpected concurrent creation error: %v", err)
		}
	}
	if successes != 1 {
		t.Fatalf("concurrent creation successes = %d, want 1", successes)
	}

	var user persistence.User
	if err := postgres.DB.First(&user, "id = ?", "create-user").Error; err != nil {
		t.Fatal(err)
	}
	var activeGames int64
	if err := postgres.DB.Model(&persistence.MineGame{}).
		Where("user_id = ? AND status = 'active'", "create-user").Count(&activeGames).Error; err != nil {
		t.Fatal(err)
	}
	if user.RemainingCoin != 400_00 || activeGames != 1 {
		t.Fatalf("balance/active games = %d/%d, want 400_00/1", user.RemainingCoin, activeGames)
	}
}

func TestStakeMineTerminalRevealAndCashOutPayOnce(t *testing.T) {
	postgres := openStakeMinePostgres(t)
	seedStakeMineUser(t, postgres, "terminal-user", 500_00)
	seedStakeMineGame(t, postgres, "terminal-game", "terminal-user", 13, 250_00)

	service := stakemine.NewService(stakemine.NewGORMRepository(postgres.DB), zap.NewNop())
	start := make(chan struct{})
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		_, _, err := service.RevealTile(context.Background(), "terminal-user", "terminal-game", stakemine.RevealInput{Index: 13})
		errs <- err
	}()
	go func() {
		defer wg.Done()
		<-start
		_, err := service.CashOut(context.Background(), "terminal-user", "terminal-game")
		errs <- err
	}()
	close(start)
	wg.Wait()
	close(errs)

	successes := 0
	for err := range errs {
		if err == nil {
			successes++
		} else if !errors.Is(err, stakemine.ErrGameConflict) {
			t.Fatalf("unexpected terminal transition error: %v", err)
		}
	}
	if successes != 1 {
		t.Fatalf("terminal transition successes = %d, want 1", successes)
	}

	var user persistence.User
	if err := postgres.DB.First(&user, "id = ?", "terminal-user").Error; err != nil {
		t.Fatal(err)
	}
	var game persistence.MineGame
	if err := postgres.DB.First(&game, "id = ?", "terminal-game").Error; err != nil {
		t.Fatal(err)
	}
	if game.Status != "won" && game.Status != "cashed_out" {
		t.Fatalf("terminal status = %q", game.Status)
	}
	if user.RemainingCoin != 500_00+game.CurrentPayout {
		t.Fatalf("balance = %d, want exactly one payout of %d", user.RemainingCoin, game.CurrentPayout)
	}
}
