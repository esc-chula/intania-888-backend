package match

func snapshotFromInput(input *Input) *Snapshot {
	item := &Snapshot{
		ID:         input.ID,
		TeamAScore: input.TeamAScore,
		TeamBScore: input.TeamBScore,
		TypeID:     input.TypeID,
		LocationID: input.LocationID,
		IsDraw:     input.IsDraw,
		StartTime:  input.StartTime,
		EndTime:    input.EndTime,
	}
	if input.TeamAID != "" {
		item.TeamAID = &input.TeamAID
	}
	if input.TeamBID != "" {
		item.TeamBID = &input.TeamBID
	}
	if input.WinnerID != "" {
		item.WinnerID = &input.WinnerID
	}

	return item
}

func resultFromSnapshot(item Snapshot) *Result {
	result := &Result{
		ID:            item.ID,
		TeamAScore:    item.TeamAScore,
		TeamBScore:    item.TeamBScore,
		TypeID:        item.TypeID,
		LocationID:    item.LocationID,
		LocationTitle: item.LocationTitle,
		IsDraw:        item.IsDraw,
		StartTime:     item.StartTime,
		EndTime:       item.EndTime,
	}
	if item.TeamAID != nil {
		result.TeamAID = *item.TeamAID
	}
	if item.TeamBID != nil {
		result.TeamBID = *item.TeamBID
	}
	if item.WinnerID != nil {
		result.WinnerID = *item.WinnerID
	}

	return result
}
