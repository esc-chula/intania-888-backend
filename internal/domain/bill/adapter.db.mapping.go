package bill

import (
	"github.com/esc-chula/intania-888-backend/internal/domain/match"
	persistence "github.com/esc-chula/intania-888-backend/internal/persistence/model"
	"github.com/esc-chula/intania-888-backend/internal/value"
)

func matchSnapshot(row persistence.Match) match.Snapshot {
	return match.Snapshot{
		ID:            row.ID,
		TeamAID:       row.TeamAID,
		TeamBID:       row.TeamBID,
		TeamAScore:    row.TeamAScore,
		TeamBScore:    row.TeamBScore,
		WinnerID:      row.WinnerID,
		TypeID:        row.TypeID,
		LocationID:    row.LocationID,
		LocationTitle: row.Location.Title,
		IsDraw:        row.IsDraw,
		StartTime:     row.StartTime,
		EndTime:       row.EndTime,
	}
}

func billFromRow(row *persistence.BillHead) *Result {
	result := &Result{
		ID:        row.ID,
		Total:     value.MustMoneyFromMinor(row.Total),
		UserID:    row.UserID,
		Status:    row.Status,
		SettledAt: row.SettledAt,
		VoidedAt:  row.VoidedAt,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
		Lines:     make([]Line, 0, len(row.Lines)),
	}
	if row.Payout != nil {
		payout := value.MustMoneyFromMinor(*row.Payout)
		result.Payout = &payout
	}
	for _, line := range row.Lines {
		result.Lines = append(result.Lines, Line{
			BillID:      line.BillID,
			MatchID:     line.MatchID,
			Rate:        value.MustRateFromMicro(line.Rate),
			BettingOn:   line.BettingOn,
			VoteColorID: line.VoteColorID,
			Match:       matchSnapshot(line.Match),
		})
	}

	return result
}

func billsFromRows(rows []*persistence.BillHead) []*Result {
	results := make([]*Result, len(rows))
	for i := range rows {
		results[i] = billFromRow(rows[i])
	}

	return results
}

func billToRow(bill *Result) persistence.BillHead {
	row := persistence.BillHead{
		ID:        bill.ID,
		Total:     bill.Total.MinorUnits(),
		UserID:    bill.UserID,
		Status:    bill.Status,
		CreatedAt: bill.CreatedAt,
		UpdatedAt: bill.UpdatedAt,
	}
	for _, line := range bill.Lines {
		row.Lines = append(row.Lines, persistence.BillLine{
			BillID:      line.BillID,
			MatchID:     line.MatchID,
			BettingOn:   line.BettingOn,
			VoteColorID: line.VoteColorID,
			Rate:        line.Rate.MicroUnits(),
		})
	}

	return row
}
