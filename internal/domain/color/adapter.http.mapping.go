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
