package bill

import "github.com/esc-chula/intania-888-backend/internal/domain/match"

func billResultDTO(result *Result) *HeadResponse {
	if result == nil {
		return nil
	}
	dto := &HeadResponse{
		ID:        result.ID,
		Total:     result.Total,
		UserID:    result.UserID,
		Status:    result.Status,
		Payout:    result.Payout,
		SettledAt: result.SettledAt,
		VoidedAt:  result.VoidedAt,
		Lines:     make([]*LineResponse, 0, len(result.Lines)),
	}
	for _, line := range result.Lines {
		dto.Lines = append(dto.Lines, &LineResponse{
			BillID:    line.BillID,
			MatchID:   line.MatchID,
			Rate:      line.Rate,
			BettingOn: line.BettingOn,
			Match:     match.ResponseFromSnapshot(line.Match),
		})
	}
	return dto
}

func billResultsDTO(results []*Result) []*HeadResponse {
	output := make([]*HeadResponse, len(results))
	for i := range results {
		output[i] = billResultDTO(results[i])
	}
	return output
}

func createInput(request CreateBillRequest) *CreateInput {
	input := &CreateInput{Total: request.Total, Lines: make([]Selection, len(request.Lines))}
	for i, line := range request.Lines {
		input.Lines[i] = Selection(line)
	}
	return input
}
