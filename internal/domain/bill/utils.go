package bill

import "github.com/wiraphatys/intania888/internal/model"

// mapBillDtoToEntity maps a BillHeadDto to a BillHead entity
func mapBillDtoToEntity(billDto *model.BillHeadDto) *model.BillHead {
	return &model.BillHead{
		Id:        billDto.Id,
		Total:     billDto.Total,
		UserId:    billDto.UserId,
		CreatedAt: billDto.CreatedAt,
		UpdatedAt: billDto.UpdatedAt,
		Lines:     mapBillLineDtoToEntity(billDto.Lines),
	}
}

// mapBillEntityToDto maps a BillHead entity to a BillHeadDto
func mapBillEntityToDto(bill *model.BillHead) *model.BillHeadDto {
	return &model.BillHeadDto{
		Id:        bill.Id,
		Total:     bill.Total,
		UserId:    bill.UserId,
		CreatedAt: bill.CreatedAt,
		UpdatedAt: bill.UpdatedAt,
		Lines:     mapBillLineEntityToDto(bill.Lines),
	}
}

// mapBillsEntityToDto maps a slice of BillHead entities to a slice of BillHeadDto
func mapBillsEntityToDto(bills []*model.BillHead) []*model.BillHeadDto {
	billDtos := make([]*model.BillHeadDto, len(bills))
	for i, bill := range bills {
		billDtos[i] = mapBillEntityToDto(bill)
	}
	return billDtos
}

// mapBillLineDtoToEntity maps a slice of BillLineDto to a slice of BillLine entities
func mapBillLineDtoToEntity(lineDtos []*model.BillLineDto) []model.BillLine {
	lines := make([]model.BillLine, len(lineDtos))
	for i, lineDto := range lineDtos {
		lines[i] = model.BillLine{
			BillId:    lineDto.BillId,
			MatchId:   lineDto.MatchId,
			Rate:      lineDto.Rate,
			BettingOn: lineDto.BettingOn,
		}
	}
	return lines
}

// mapBillLineEntityToDto maps a slice of BillLine entities to a slice of BillLineDto
func mapBillLineEntityToDto(lines []model.BillLine) []*model.BillLineDto {
	lineDtos := make([]*model.BillLineDto, len(lines))
	for i, line := range lines {
		lineDtos[i] = &model.BillLineDto{
			BillId:    line.BillId,
			MatchId:   line.MatchId,
			Rate:      line.Rate,
			BettingOn: line.BettingOn,
			CreatedAt: line.CreatedAt,
			UpdatedAt: line.UpdatedAt,
			Match: model.MatchDto{
				Id:        line.Match.Id,
				TeamAId:   line.Match.TeamA_Id,
				TeamBId:   line.Match.TeamB_Id,
				WinnerId:  line.Match.WinnerId,
				TypeId:    line.Match.TypeId,
				StartTime: line.Match.StartTime,
				EndTime:   line.Match.EndTime,
			},
			Color: model.ColorDto{
				Id: line.Color.Id,
			},
		}
	}
	return lineDtos
}
