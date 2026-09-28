package match

import (
	persistence "github.com/esc-chula/intania-888-backend/internal/persistence/model"
)

func snapshotFromRow(row persistence.Match) Snapshot {
	return Snapshot{
		ID:         row.ID,
		TeamAID:    row.TeamAID,
		TeamBID:    row.TeamBID,
		TeamAScore: row.TeamAScore,
		TeamBScore: row.TeamBScore,
		WinnerID:   row.WinnerID,
		TypeID:     row.TypeID,
		IsDraw:     row.IsDraw,
		StartTime:  row.StartTime,
		EndTime:    row.EndTime,
	}
}

func snapshotToRow(item *Snapshot) persistence.Match {
	return persistence.Match{
		ID:         item.ID,
		TeamAID:    item.TeamAID,
		TeamBID:    item.TeamBID,
		TeamAScore: item.TeamAScore,
		TeamBScore: item.TeamBScore,
		WinnerID:   item.WinnerID,
		TypeID:     item.TypeID,
		IsDraw:     item.IsDraw,
		StartTime:  item.StartTime,
		EndTime:    item.EndTime,
	}
}
