package location

func responses(rows []*Location) []*Response {
	result := make([]*Response, len(rows))
	for i, row := range rows {
		result[i] = response(row)
	}

	return result
}

func response(row *Location) *Response {
	if row == nil {
		return nil
	}

	return &Response{
		ID:    row.ID,
		Title: row.Title,
	}
}
