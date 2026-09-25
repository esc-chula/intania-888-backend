//go:build integration

package integration_test

import (
	"errors"
	"os"
	"sync"
	"testing"

	"github.com/esc-chula/intania-888-backend/internal/domain/stakemine"
	"github.com/esc-chula/intania-888-backend/internal/model"
	"github.com/esc-chula/intania-888-backend/internal/testutil"
	"go.uber.org/zap"
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

	grid := make([]stakemine.Tile, 16)
	for i := range grid {
		grid[i] = stakemine.Tile{Index: i, Type: "diamond", Revealed: i < revealed}
	}

	grid[14].Type = "bomb"
	grid[15].Type = "bomb"

	gridJSON, err := stakemine.GridToJSON(grid)
	if err != nil {
		t.Fatal(err)
	}

	return gridJSON
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
		 VALUES($1, $2, 10000, 'low', 'active', $3, $4, 1000000, $5)`,
		gameID,
		userID,
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
	seedStakeMineUser(t, postgres, "cashout-user", 50000)
	seedStakeMineGame(t, postgres, "cashout-game", "cashout-user", 1, 25000)

	service := stakemine.NewStakeMineService(
		stakemine.NewStakeMineRepository(postgres.DB),
		postgres.DB,
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
			_, err := service.CashOut("cashout-user", "cashout-game")
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

	var user model.User
	if err := postgres.DB.First(&user, "id = ?", "cashout-user").Error; err != nil {
		t.Fatal(err)
	}

	if user.RemainingCoin != 75000 {
		t.Fatalf("user balance = %d, want 75000", user.RemainingCoin)
	}

	var game model.MineGame
	if err := postgres.DB.First(&game, "id = ?", "cashout-game").Error; err != nil {
		t.Fatal(err)
	}

	if game.Status != "cashed_out" {
		t.Fatalf("game status = %q, want cashed_out", game.Status)
	}
}

func TestStakeMineHistoryFailureRollsBackTerminalPayout(t *testing.T) {
	postgres := openStakeMinePostgres(t)
	seedStakeMineUser(t, postgres, "history-user", 50000)
	seedStakeMineGame(t, postgres, "history-game", "history-user", 13, 25000)

	cleanup, err := postgres.InstallFailureTrigger("mine_game_histories", "INSERT")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := cleanup(); err != nil {
			t.Errorf("remove failure trigger: %v", err)
		}
	}()

	service := stakemine.NewStakeMineService(
		stakemine.NewStakeMineRepository(postgres.DB),
		postgres.DB,
		zap.NewNop(),
	)

	_, _, err = service.RevealTile("history-user", "history-game", &model.RevealMineTileRequest{Index: 13})
	if err == nil {
		t.Fatal("terminal reveal succeeded despite history failure")
	}

	var user model.User
	if err := postgres.DB.First(&user, "id = ?", "history-user").Error; err != nil {
		t.Fatal(err)
	}

	if user.RemainingCoin != 50000 {
		t.Fatalf("user balance = %d, want 50000", user.RemainingCoin)
	}

	var game model.MineGame
	if err := postgres.DB.First(&game, "id = ?", "history-game").Error; err != nil {
		t.Fatal(err)
	}

	if game.Status != "active" {
		t.Fatalf("game status = %q, want active", game.Status)
	}
}
