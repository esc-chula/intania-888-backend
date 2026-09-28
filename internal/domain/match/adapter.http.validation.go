package match

import (
	"strings"

	"github.com/esc-chula/intania-888-backend/internal/apierror"
)

// ValidateRequest validates the HTTP request fields.
func (r CreateMatchRequest) ValidateRequest() map[string]string {
	details := make(map[string]string)
	if strings.TrimSpace(r.TeamAID) == "" {
		details["team_a"] = "is required"
	}
	if strings.TrimSpace(r.TeamBID) == "" {
		details["team_b"] = "is required"
	}
	if strings.TrimSpace(r.TypeID) == "" {
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

// ValidateRequest validates the HTTP request fields.
func (r UpdateMatchRequest) ValidateRequest() map[string]string {
	details := make(map[string]string)
	if r.TeamAID == nil && r.TeamBID == nil && r.TypeID == nil && r.StartTime == nil && r.EndTime == nil {
		details["body"] = "must include at least one match field"
		return details
	}
	if r.TeamAID != nil && strings.TrimSpace(*r.TeamAID) == "" {
		details["team_a"] = "must not be empty"
	}
	if r.TeamBID != nil && strings.TrimSpace(*r.TeamBID) == "" {
		details["team_b"] = "must not be empty"
	}
	if r.TypeID != nil && strings.TrimSpace(*r.TypeID) == "" {
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

// ValidateRequest validates the HTTP request fields.
func (r ScoreDTO) ValidateRequest() map[string]string {
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

// UnmarshalJSON rejects unknown fields and records required zero values.
func (r *ScoreDTO) UnmarshalJSON(data []byte) error {
	type wire struct {
		TeamAScore *int `json:"team_a_score"`
		TeamBScore *int `json:"team_b_score"`
	}
	var value wire
	if err := apierror.DecodeKnownObject(data, &value, "team_a_score", "team_b_score"); err != nil {
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

// ValidateRequest validates the HTTP request fields.
func (r ResultRequest) ValidateRequest() map[string]string {
	details := make(map[string]string)
	switch r.Outcome {
	case "winner":
		if r.WinnerID == nil || strings.TrimSpace(*r.WinnerID) == "" {
			details["winner_id"] = "is required when outcome is winner"
		}
	case "draw":
		if r.WinnerID != nil {
			details["winner_id"] = "must be omitted when outcome is draw"
		}
	default:
		details["outcome"] = "must be winner or draw"
	}
	return details
}
