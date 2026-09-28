package event

func scheduleToResponse(schedule *DailyRewardSchedule) *DailyRewardScheduleResponse {
	if schedule == nil {
		return nil
	}
	overrides := make([]DailyRewardScheduleItemResponse, len(schedule.Overrides))
	for i, item := range schedule.Overrides {
		overrides[i] = DailyRewardScheduleItemResponse(item)
	}
	return &DailyRewardScheduleResponse{DefaultAmount: schedule.DefaultAmount, Overrides: overrides}
}

func spinToResponse(result *SpinResult) *SpinResponse {
	if result == nil {
		return nil
	}
	response := &SpinResponse{Slots: result.Slots, Reward: result.Reward}
	if result.StealToken != nil {
		token := result.StealToken
		response.StealToken = &StealTokenResponse{
			Token:       token.Token,
			ExpiresAt:   token.ExpiresAt,
			VictimCount: token.VictimCount,
			Message:     token.Message,
		}
		response.Candidates = make([]CandidatePreviewResponse, len(result.Candidates))
		for i, candidate := range result.Candidates {
			response.Candidates[i] = CandidatePreviewResponse(candidate)
		}
	}
	return response
}

func stealToResponse(result *StealResult) *UseStealTokenResponse {
	if result == nil {
		return nil
	}
	candidates := make([]VictimDetailResponse, len(result.AllCandidates))
	for i, candidate := range result.AllCandidates {
		candidates[i] = VictimDetailResponse(candidate)
	}
	return &UseStealTokenResponse{TotalStolen: result.TotalStolen, RaiderNewBalance: result.RaiderNewBalance,
		AllCandidates: candidates, Message: result.Message}
}
