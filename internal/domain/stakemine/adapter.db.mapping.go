package stakemine

import (
	"encoding/json"
	"fmt"

	persistence "github.com/esc-chula/intania-888-backend/internal/persistence/model"
)

type storedTile struct {
	Index    int    `json:"index"`
	Type     string `json:"type"`
	Revealed bool   `json:"revealed"`
}

func gridToJSON(grid []Tile) (string, error) {
	if grid == nil {
		return "null", nil
	}
	stored := make([]storedTile, len(grid))
	for i, tile := range grid {
		stored[i] = storedTile(tile)
	}
	data, err := json.Marshal(stored)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func jsonToGrid(data string) ([]Tile, error) {
	var stored []storedTile
	if err := json.Unmarshal([]byte(data), &stored); err != nil {
		return nil, err
	}
	if stored == nil {
		return nil, nil
	}
	grid := make([]Tile, len(stored))
	for i, tile := range stored {
		grid[i] = Tile(tile)
	}
	return grid, nil
}

func gameSnapshot(row *persistence.MineGame, includeGrid bool) (*Game, error) {
	var grid []Tile
	var gridErr error
	if includeGrid {
		grid, gridErr = jsonToGrid(row.GridData)
	}
	return &Game{
		ID:            row.ID,
		UserID:        row.UserID,
		BetAmount:     row.BetAmount,
		RiskLevel:     row.RiskLevel,
		Status:        row.Status,
		RevealedCount: row.RevealedCount,
		CurrentPayout: row.CurrentPayout,
		Multiplier:    row.Multiplier,
		Grid:          grid,
		GridError:     gridErr,
		CreatedAt:     row.CreatedAt,
		UpdatedAt:     row.UpdatedAt,
		CompletedAt:   row.CompletedAt,
	}, nil
}

func gameRow(game *Game) (*persistence.MineGame, error) {
	gridJSON, err := gridToJSON(game.Grid)
	if err != nil {
		return nil, fmt.Errorf("serialize game grid: %w", err)
	}
	return &persistence.MineGame{
		ID:            game.ID,
		UserID:        game.UserID,
		BetAmount:     game.BetAmount,
		RiskLevel:     game.RiskLevel,
		Status:        game.Status,
		RevealedCount: game.RevealedCount,
		CurrentPayout: game.CurrentPayout,
		Multiplier:    game.Multiplier,
		GridData:      gridJSON,
		CreatedAt:     game.CreatedAt,
		UpdatedAt:     game.UpdatedAt,
		CompletedAt:   game.CompletedAt,
	}, nil
}
