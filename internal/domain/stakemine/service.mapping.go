package stakemine

import "github.com/esc-chula/intania-888-backend/internal/value"

func gameResult(game *Game, hideUnrevealed bool) *GameResult {
	safeGrid := GetSafeGrid(game.Grid, hideUnrevealed)
	tiles := make([]TileResult, len(safeGrid))
	for i, tile := range safeGrid {
		tiles[i] = TileResult(tile)
	}
	return &GameResult{
		ID:            game.ID,
		UserID:        game.UserID,
		BetAmount:     value.MustMoneyFromMinor(game.BetAmount),
		RiskLevel:     game.RiskLevel,
		Grid:          tiles,
		RevealedCount: game.RevealedCount,
		CurrentPayout: value.MustMoneyFromMinor(game.CurrentPayout),
		Multiplier:    value.MustRateFromMicro(game.Multiplier),
		Status:        game.Status,
		CreatedAt:     game.CreatedAt,
		CompletedAt:   game.CompletedAt,
	}
}
