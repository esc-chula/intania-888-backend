package stakemine

import (
	"testing"

	persistence "github.com/esc-chula/intania-888-backend/internal/persistence/model"
)

func TestGameSnapshotDefersMalformedGridFailure(t *testing.T) {
	game, err := gameSnapshot(&persistence.MineGame{ID: "game", UserID: "owner", Status: "lost", GridData: "broken"}, true)
	if err != nil || game == nil || game.UserID != "owner" || game.Status != "lost" || game.GridError == nil {
		t.Fatalf("snapshot = %#v, err = %v", game, err)
	}
}
