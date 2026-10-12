package stakemine

func gameResponse(game *GameResult) *GameResponse {
	if game == nil {
		return nil
	}
	tiles := make([]TileResponse, len(game.Grid))
	for i, tile := range game.Grid {
		tiles[i] = TileResponse(tile)
	}

	return &GameResponse{
		ID:            game.ID,
		UserID:        game.UserID,
		BetAmount:     game.BetAmount,
		RiskLevel:     game.RiskLevel,
		Grid:          tiles,
		RevealedCount: game.RevealedCount,
		CurrentPayout: game.CurrentPayout,
		Multiplier:    game.Multiplier,
		Status:        game.Status,
		CreatedAt:     game.CreatedAt,
		CompletedAt:   game.CompletedAt,
	}
}

func historyResponses(history []HistoryResult) []HistoryResponse {
	result := make([]HistoryResponse, len(history))
	for i, game := range history {
		result[i] = HistoryResponse(game)
	}

	return result
}

func statsResponse(stats *StatsResult) *StatsResponse {
	if stats == nil {
		return nil
	}

	return &StatsResponse{
		TotalGames:          stats.TotalGames,
		GamesWon:            stats.GamesWon,
		GamesLost:           stats.GamesLost,
		GamesCashedOut:      stats.GamesCashedOut,
		TotalWagered:        stats.TotalWagered,
		TotalWinnings:       stats.TotalWinnings,
		NetProfit:           stats.NetProfit,
		ActiveWagered:       stats.ActiveWagered,
		ActiveCurrentPayout: stats.ActiveCurrentPayout,
		WinRate:             stats.WinRate,
	}
}
