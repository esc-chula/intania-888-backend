package stakemine

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"go.uber.org/zap"

	"github.com/esc-chula/intania-888-backend/internal/value"
)

type fakeGameRepository struct {
	Repository
	game         *Game
	balance      int64
	activeCount  int64
	history      []History
	historyError error
	adjustError  error
	calls        []string
	contexts     []context.Context
}

func (r *fakeGameRepository) WithinTransaction(ctx context.Context, fn func(Transaction) error) error {
	r.contexts = append(r.contexts, ctx)
	r.calls = append(r.calls, "begin")
	originalBalance := r.balance
	var originalGame *Game
	if r.game != nil {
		copy := *r.game
		copy.Grid = append([]Tile(nil), r.game.Grid...)
		originalGame = &copy
	}
	originalHistory := append([]History(nil), r.history...)
	if err := fn(r); err != nil {
		r.game = originalGame
		r.balance = originalBalance
		r.history = originalHistory
		r.calls = append(r.calls, "rollback")
		return err
	}
	r.calls = append(r.calls, "commit")
	return nil
}

func (r *fakeGameRepository) LockUserBalance(ctx context.Context, _ string) (int64, error) {
	r.contexts = append(r.contexts, ctx)
	r.calls = append(r.calls, "lock account")
	return r.balance, nil
}

func (r *fakeGameRepository) CountActiveGames(ctx context.Context, _ string) (int64, error) {
	r.contexts = append(r.contexts, ctx)
	r.calls = append(r.calls, "count active")
	return r.activeCount, nil
}

func (r *fakeGameRepository) LockGame(ctx context.Context, _ string) (*Game, error) {
	r.contexts = append(r.contexts, ctx)
	r.calls = append(r.calls, "lock game")
	copy := *r.game
	copy.Grid = append([]Tile(nil), r.game.Grid...)
	return &copy, nil
}

func (r *fakeGameRepository) CreateGame(ctx context.Context, game *Game) error {
	r.contexts = append(r.contexts, ctx)
	r.calls = append(r.calls, "create game")
	r.game = game
	return nil
}

func (r *fakeGameRepository) SaveGame(ctx context.Context, game *Game) error {
	r.contexts = append(r.contexts, ctx)
	r.calls = append(r.calls, "save game")
	r.game = game
	return nil
}

func (r *fakeGameRepository) AdjustBalance(ctx context.Context, _ string, amount int64) error {
	r.contexts = append(r.contexts, ctx)
	r.calls = append(r.calls, "adjust balance")
	if r.adjustError != nil {
		return r.adjustError
	}
	r.balance += amount
	return nil
}

func (r *fakeGameRepository) CreateHistory(ctx context.Context, history *History) error {
	r.contexts = append(r.contexts, ctx)
	r.calls = append(r.calls, "create history")
	if r.historyError != nil {
		return r.historyError
	}
	r.history = append(r.history, *history)
	return nil
}

func testGame(revealed int) *Game {
	grid := make([]Tile, 16)
	for i := range grid {
		grid[i] = Tile{Index: i, Type: "diamond", Revealed: i < revealed}
	}
	grid[14].Type = "bomb"
	grid[15].Type = "bomb"
	return &Game{ID: "game", UserID: "owner", BetAmount: 10000, CurrentPayout: 25000, Multiplier: 1000000, RiskLevel: "low", Status: "active", Grid: grid, RevealedCount: revealed}
}

func TestCreateGameLocksAccountAndReturnsHiddenTiles(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	repo := &fakeGameRepository{balance: 50000}
	game, err := NewService(repo, zap.NewNop()).CreateGame(ctx, "owner", CreateInput{BetAmount: value.MustMoneyFromMinor(10000), RiskLevel: "low"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"begin", "lock account", "count active", "create game", "adjust balance", "commit"}
	if !reflect.DeepEqual(repo.calls, want) || repo.balance != 40000 {
		t.Fatalf("calls/balance = %v %d", repo.calls, repo.balance)
	}
	for _, received := range repo.contexts {
		if received != ctx {
			t.Fatal("request context was not propagated")
		}
	}
	for _, tile := range game.Grid {
		if tile.Type != "hidden" || tile.Revealed {
			t.Fatalf("active tile exposed: %+v", tile)
		}
	}
}

func TestCreateGameChecksExistingActiveGameBeforeBalanceMutation(t *testing.T) {
	repo := &fakeGameRepository{balance: 50000, activeCount: 1}
	_, err := NewService(repo, zap.NewNop()).CreateGame(context.Background(), "owner", CreateInput{BetAmount: value.MustMoneyFromMinor(10000), RiskLevel: "low"})
	if !errors.Is(err, ErrGameConflict) || repo.balance != 50000 || repo.game != nil {
		t.Fatalf("existing active game caused mutation: err=%v repo=%+v", err, repo)
	}
}

func TestTerminalRevealHistoryFailureRollsBackPayoutAndState(t *testing.T) {
	failure := errors.New("history unavailable")
	repo := &fakeGameRepository{game: testGame(13), balance: 50000, historyError: failure}
	_, _, err := NewService(repo, zap.NewNop()).RevealTile(context.Background(), "owner", "game", RevealInput{Index: 13})
	if !errors.Is(err, failure) || repo.balance != 50000 || repo.game.Status != "active" || repo.game.Grid[13].Revealed || len(repo.history) != 0 {
		t.Fatalf("failed terminal reveal changed state: err=%v balance=%d game=%+v", err, repo.balance, repo.game)
	}
	want := []string{"begin", "lock game", "save game", "adjust balance", "create history", "rollback"}
	if !reflect.DeepEqual(repo.calls, want) {
		t.Fatalf("calls = %v", repo.calls)
	}
}

func TestCashOutPreservesOwnershipConflictAndAtomicFailure(t *testing.T) {
	failure := errors.New("credit unavailable")
	tests := []struct {
		name        string
		actor       string
		revealed    int
		adjustError error
		wantError   error
	}{
		{"other owner", "other", 1, nil, ErrGameForbidden},
		{"no revealed tile", "owner", 0, nil, ErrGameConflict},
		{"credit failure", "owner", 1, failure, failure},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo := &fakeGameRepository{game: testGame(test.revealed), balance: 50000, adjustError: test.adjustError}
			_, err := NewService(repo, zap.NewNop()).CashOut(context.Background(), test.actor, "game")
			if !errors.Is(err, test.wantError) || repo.balance != 50000 || repo.game.Status != "active" {
				t.Fatalf("cash-out failure changed state: err=%v balance=%d game=%+v", err, repo.balance, repo.game)
			}
		})
	}
}
