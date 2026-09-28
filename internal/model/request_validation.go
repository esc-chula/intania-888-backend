package model

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
)

func (r *ScoreDto) UnmarshalJSON(data []byte) error {
	type wire struct {
		TeamAScore *int `json:"team_a_score"`
		TeamBScore *int `json:"team_b_score"`
	}
	var value wire
	if err := decodeKnownObject(data, &value, "team_a_score", "team_b_score"); err != nil {
		return err
	}
	r.teamAScoreSet = value.TeamAScore != nil
	r.teamBScoreSet = value.TeamBScore != nil
	if value.TeamAScore != nil {
		r.TeamAScore = *value.TeamAScore
	}
	if value.TeamBScore != nil {
		r.TeamBScore = *value.TeamBScore
	}
	return nil
}

func (r ScoreDto) ValidateRequest() map[string]string {
	details := make(map[string]string)
	if !r.teamAScoreSet {
		details["team_a_score"] = "is required"
	} else if r.TeamAScore < 0 {
		details["team_a_score"] = "must be zero or greater"
	}
	if !r.teamBScoreSet {
		details["team_b_score"] = "is required"
	} else if r.TeamBScore < 0 {
		details["team_b_score"] = "must be zero or greater"
	}
	return details
}

func (r *RevealMineTileRequest) UnmarshalJSON(data []byte) error {
	var value struct {
		Index *int `json:"index"`
	}
	if err := decodeKnownObject(data, &value, "index"); err != nil {
		return err
	}
	r.indexSet = value.Index != nil
	if value.Index != nil {
		r.Index = *value.Index
	}
	return nil
}

func (r RevealMineTileRequest) ValidateRequest() map[string]string {
	if !r.indexSet || r.Index < 0 || r.Index > 15 {
		return map[string]string{"index": "must be between 0 and 15"}
	}
	return nil
}

func (r *UseStealTokenRequestDto) UnmarshalJSON(data []byte) error {
	var value struct {
		Token       string `json:"token"`
		VictimIndex *int   `json:"victim_index"`
	}
	if err := decodeKnownObject(data, &value, "token", "victim_index"); err != nil {
		return err
	}
	r.Token = value.Token
	r.victimIndexSet = value.VictimIndex != nil
	if value.VictimIndex != nil {
		r.VictimIndex = *value.VictimIndex
	}
	return nil
}

func (r UseStealTokenRequestDto) ValidateRequest() map[string]string {
	details := make(map[string]string)
	if strings.TrimSpace(r.Token) == "" {
		details["token"] = "is required"
	}
	if !r.victimIndexSet || r.VictimIndex < 0 {
		details["victim_index"] = "must be zero or greater"
	}
	return details
}

func (r CreateMineGameRequest) ValidateRequest() map[string]string {
	details := make(map[string]string)
	if r.BetAmount.MinorUnits() < 100 || r.BetAmount.MinorUnits() > 100_000_000 {
		details["bet_amount"] = "must be between 1 and 1000000"
	}
	switch r.RiskLevel {
	case "low", "medium", "high":
	default:
		details["risk_level"] = "must be low, medium, or high"
	}
	return details
}

func (r CreateBillRequest) ValidateRequest() map[string]string {
	details := make(map[string]string)
	if r.Total.IsZero() {
		details["total"] = "must be greater than zero"
	}
	if len(r.Lines) == 0 {
		details["lines"] = "must contain at least one selection"
	}
	for _, line := range r.Lines {
		if strings.TrimSpace(line.MatchId) == "" {
			details["lines"] = "each selection must include a match_id"
			break
		}
		if strings.TrimSpace(line.BettingOn) == "" {
			details["lines"] = "each selection must include a betting_on value"
			break
		}
	}
	return details
}

func (r VoidBillRequest) ValidateRequest() map[string]string {
	if strings.TrimSpace(r.Reason) == "" {
		return map[string]string{"reason": "is required"}
	}
	return nil
}

func (r MatchResultRequest) ValidateRequest() map[string]string {
	details := make(map[string]string)
	switch r.Outcome {
	case "winner":
		if r.WinnerId == nil || strings.TrimSpace(*r.WinnerId) == "" {
			details["winner_id"] = "is required when outcome is winner"
		}
	case "draw":
		if r.WinnerId != nil {
			details["winner_id"] = "must be omitted when outcome is draw"
		}
	default:
		details["outcome"] = "must be winner or draw"
	}
	return details
}

func (r CreateMatchRequest) ValidateRequest() map[string]string {
	details := make(map[string]string)
	if strings.TrimSpace(r.TeamAId) == "" {
		details["team_a"] = "is required"
	}
	if strings.TrimSpace(r.TeamBId) == "" {
		details["team_b"] = "is required"
	}
	if strings.TrimSpace(r.TypeId) == "" {
		details["type"] = "is required"
	}
	if r.StartTime.IsZero() {
		details["start_time"] = "is required"
	}
	if r.EndTime.IsZero() || !r.EndTime.After(r.StartTime) {
		details["end_time"] = "must be after start_time"
	}
	return details
}

func (r UpdateMatchRequest) ValidateRequest() map[string]string {
	details := make(map[string]string)
	if r.TeamAId == nil && r.TeamBId == nil && r.TypeId == nil && r.StartTime == nil && r.EndTime == nil {
		details["body"] = "must include at least one match field"
		return details
	}
	if r.TeamAId != nil && strings.TrimSpace(*r.TeamAId) == "" {
		details["team_a"] = "must not be empty"
	}
	if r.TeamBId != nil && strings.TrimSpace(*r.TeamBId) == "" {
		details["team_b"] = "must not be empty"
	}
	if r.TypeId != nil && strings.TrimSpace(*r.TypeId) == "" {
		details["type"] = "must not be empty"
	}
	if r.StartTime != nil && r.StartTime.IsZero() {
		details["start_time"] = "must be a valid timestamp"
	}
	if r.EndTime != nil && r.EndTime.IsZero() {
		details["end_time"] = "must be a valid timestamp"
	}
	return details
}

func (r DeductCoinRequest) ValidateRequest() map[string]string {
	if r.Amount.MinorUnits() < 100 || r.Amount.MinorUnits() > 100_000_000 {
		return map[string]string{"amount": "must be between 1 and 1000000"}
	}
	return nil
}

func (r SetDailyRewardRequest) ValidateRequest() map[string]string {
	if !r.amountSet {
		return map[string]string{"amount": "is required"}
	}
	if r.Amount.MinorUnits() < 0 {
		return map[string]string{"amount": "must be zero or greater"}
	}
	return nil
}

func (r *SetDailyRewardRequest) UnmarshalJSON(data []byte) error {
	var value struct {
		Amount *Money `json:"amount"`
	}
	if err := decodeKnownObject(data, &value, "amount"); err != nil {
		return err
	}
	r.amountSet = value.Amount != nil
	if value.Amount != nil {
		r.Amount = *value.Amount
	}
	return nil
}

func (r UpdateUserDto) ValidateRequest() map[string]string {
	if strings.TrimSpace(r.Name) == "" {
		return map[string]string{"name": "is required"}
	}
	return nil
}

func (r AdminUpdateUserDto) ValidateRequest() map[string]string {
	if strings.TrimSpace(r.Name) == "" {
		return map[string]string{"name": "is required"}
	}
	return nil
}

func StrictJSONObject(data []byte) bool {
	return len(bytes.TrimSpace(data)) > 0 && bytes.TrimSpace(data)[0] == '{'
}

func decodeKnownObject(data []byte, dst any, allowed ...string) error {
	if !StrictJSONObject(data) {
		return errors.New("request must be a JSON object")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	allowedFields := make(map[string]struct{}, len(allowed))
	for _, field := range allowed {
		allowedFields[field] = struct{}{}
	}
	for field := range fields {
		if _, ok := allowedFields[field]; !ok {
			return errors.New("unknown field")
		}
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	return decoder.Decode(dst)
}
