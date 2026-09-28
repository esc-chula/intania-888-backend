package sporttype

func responses(rows []*SportType) []*Response {
	result := make([]*Response, len(rows))
	for i, row := range rows {
		result[i] = &Response{ID: row.ID, Title: row.Title}
	}
	return result
}
