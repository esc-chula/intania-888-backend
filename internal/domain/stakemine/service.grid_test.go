package stakemine

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/zap"
)

type gridReadRepository struct{ *fakeGameRepository }

func (r gridReadRepository) FindByID(context.Context, string) (*Game, error) { return r.game, nil }
func (r gridReadRepository) FindActiveByUserID(context.Context, string) (*Game, error) {
	return r.game, nil
}

func TestMalformedGridPreservesOwnershipAndStateErrors(t *testing.T) {
	decodeErr := errors.New("stored grid malformed")
	for _, tc := range []struct {
		name, user, status string
		revealed           int
		want               error
	}{
		{"other owner", "other", "active", 1, ErrGameForbidden},
		{"terminal owned", "owner", "lost", 1, ErrGameConflict},
		{"unrevealed owned", "owner", "active", 0, ErrGameConflict},
		{"eligible owned", "owner", "active", 1, decodeErr},
	} {
		t.Run(tc.name, func(t *testing.T) {
			game := testGame(tc.revealed)
			game.Status = tc.status
			game.GridError = decodeErr
			repo := &fakeGameRepository{game: game, balance: 50000}
			svc := NewService(gridReadRepository{repo}, zap.NewNop())
			_, err := svc.CashOut(context.Background(), tc.user, "game")
			if !errors.Is(err, tc.want) {
				t.Fatalf("cashout error %v; want %v", err, tc.want)
			}
			if repo.balance != 50000 || repo.game.Status != tc.status {
				t.Fatal("failed cashout changed state")
			}
			wantReveal := tc.want
			if tc.user == "owner" && tc.status == "active" {
				wantReveal = decodeErr
			}
			_, _, err = svc.RevealTile(context.Background(), tc.user, "game", RevealInput{Index: 1})
			if !errors.Is(err, wantReveal) {
				t.Fatalf("reveal error %v; want %v", err, wantReveal)
			}
			wantRead := decodeErr
			if tc.user != "owner" {
				wantRead = ErrGameForbidden
			}
			_, err = svc.GetGame(context.Background(), tc.user, "game")
			if !errors.Is(err, wantRead) {
				t.Fatalf("read error %v; want %v", err, wantRead)
			}
		})
	}
}
