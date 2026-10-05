package color

func responses(rows []*Leaderboard) []*Response {
	result := make([]*Response, len(rows))
	for i, row := range rows {
		result[i] = &Response{
			ID:         row.ID,
			Title:      row.Title,
			Won:        row.Won,
			Drawn:      row.Drawn,
			Lost:       row.Lost,
			TotalMatch: row.TotalMatch,
		}
	}

	return result
}

func coinRankResponses(rows []*CoinRank) []*CoinRankResponse {
	result := make([]*CoinRankResponse, len(rows))
	for i, row := range rows {
		result[i] = &CoinRankResponse{
			Rank:        row.Rank,
			ID:          row.ID,
			Title:       row.Title,
			TotalCoin:   row.TotalCoin,
			MemberCount: row.MemberCount,
		}
	}

	return result
}

func predictionRankResponses(rows []*PredictionRank) []*PredictionRankResponse {
	result := make([]*PredictionRankResponse, len(rows))
	for i, row := range rows {
		result[i] = &PredictionRankResponse{
			Rank:     row.Rank,
			ID:       row.ID,
			Title:    row.Title,
			Correct:  row.Correct,
			Wrong:    row.Wrong,
			Total:    row.Total,
			Accuracy: row.Accuracy,
		}
	}

	return result
}
