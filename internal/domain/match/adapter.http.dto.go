package match

import (
	"time"

	"github.com/esc-chula/intania-888-backend/internal/value"
)

// Response is the match HTTP response.
type Response struct {
	ID         string     `json:"id"`
	TeamAID    string     `json:"team_a"`
	TeamBID    string     `json:"team_b"`
	TeamAScore *int       `json:"team_a_score"`
	TeamBScore *int       `json:"team_b_score"`
	TeamARate  value.Rate `json:"team_a_rate" swaggertype:"number"`
	TeamBRate  value.Rate `json:"team_b_rate" swaggertype:"number"`
	WinnerID   string     `json:"winner"`
	TypeID     string     `json:"type"`
	IsDraw     bool       `json:"is_draw"`
	StartTime  time.Time  `json:"start_time"`
	EndTime    time.Time  `json:"end_time"`
} // @name model.MatchDto

// CreateMatchRequest is the match HTTP request.
type CreateMatchRequest struct {
	TeamAID   string    `json:"team_a" validate:"required"`
	TeamBID   string    `json:"team_b" validate:"required"`
	TypeID    string    `json:"type" validate:"required"`
	StartTime time.Time `json:"start_time" validate:"required"`
	EndTime   time.Time `json:"end_time" validate:"required"`
} // @name model.CreateMatchRequest

// UpdateMatchRequest is the match HTTP request.
type UpdateMatchRequest struct {
	TeamAID   *string    `json:"team_a"`
	TeamBID   *string    `json:"team_b"`
	TypeID    *string    `json:"type"`
	StartTime *time.Time `json:"start_time"`
	EndTime   *time.Time `json:"end_time"`
} // @name model.UpdateMatchRequest

// MatchesByType is the match HTTP response.
type MatchesByType struct {
	SportType string      `json:"sportType"`
	Matches   []*Response `json:"matches"`
} // @name model.MatchesByType

// MatchesByDate is the match HTTP response.
type MatchesByDate struct {
	Date  time.Time       `json:"date"`
	Types []MatchesByType `json:"types"`
} // @name model.MatchesByDate

// ScoreDTO is the match HTTP request.
type ScoreDTO struct {
	TeamAScore    int `json:"team_a_score" validate:"required,min=0"`
	TeamBScore    int `json:"team_b_score" validate:"required,min=0"`
	teamAScoreSet bool
	teamBScoreSet bool
} // @name model.ScoreDto

// ResultRequest is the match HTTP request.
type ResultRequest struct {
	Outcome  string  `json:"outcome" validate:"required,oneof=winner draw"`
	WinnerID *string `json:"winner_id,omitempty"`
} // @name model.MatchResultRequest
