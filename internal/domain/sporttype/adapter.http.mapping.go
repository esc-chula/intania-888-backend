package sporttype

func responses(rows []*SportType) []*Response {
	result := make([]*Response, len(rows))
	for i, row := range rows {
		result[i] = response(row)
	}
	return result
}

func response(row *SportType) *Response {
	return &Response{ID: row.ID, Title: row.Title}
}
