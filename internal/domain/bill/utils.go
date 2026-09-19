package bill

import "github.com/esc-chula/intania-888-backend/internal/model"

func mapBillEntityToDto(v *model.BillHead) *model.BillHeadDto {
	d := &model.BillHeadDto{
		Id:        v.Id,
		Total:     model.MustMoneyFromMinor(v.Total),
		UserId:    v.UserId,
		Status:    v.Status,
		SettledAt: v.SettledAt,
		VoidedAt:  v.VoidedAt,
		Lines:     make([]*model.BillLineDto, 0, len(v.Lines)),
	}

	if v.Payout != nil {
		p := model.MustMoneyFromMinor(*v.Payout)
		d.Payout = &p
	}

	for _, line := range v.Lines {
		l := &model.BillLineDto{
			BillId:    line.BillId,
			MatchId:   line.MatchId,
			Rate:      model.MustRateFromMicro(line.Rate),
			BettingOn: line.BettingOn,
		}

		l.Match = *billMatchDto(line.Match)
		d.Lines = append(d.Lines, l)
	}

	return d
}

func billMatchDto(v model.Match) *model.MatchDto {
	d := &model.MatchDto{
		Id:         v.Id,
		TeamAScore: v.TeamA_Score,
		TeamBScore: v.TeamB_Score,
		TypeId:     v.TypeId,
		IsDraw:     v.IsDraw,
		StartTime:  v.StartTime,
		EndTime:    v.EndTime,
	}

	if v.TeamA_Id != nil {
		d.TeamAId = *v.TeamA_Id
	}

	if v.TeamB_Id != nil {
		d.TeamBId = *v.TeamB_Id
	}

	if v.WinnerId != nil {
		d.WinnerId = *v.WinnerId
	}

	return d
}

func mapBillsEntityToDto(v []*model.BillHead) []*model.BillHeadDto {
	out := make([]*model.BillHeadDto, len(v))

	for i := range v {
		out[i] = mapBillEntityToDto(v[i])
	}

	return out
}
