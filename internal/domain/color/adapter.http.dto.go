package color

import "github.com/esc-chula/intania-888-backend/internal/value"

// Response is the HTTP representation of a color's completed-match standings.
type Response struct {
	ID         string `json:"id"`
	Title      string `json:"title,omitempty"`
	Won        int64  `json:"won"`
	Drawn      int64  `json:"drawn"`
	Lost       int64  `json:"lost"`
	TotalMatch int64  `json:"total_matches"`
}

// CoinRankResponse is the HTTP representation of a color's coin ranking row.
type CoinRankResponse struct {
	Rank        int         `json:"rank"`
	ID          string      `json:"id"`
	Title       string      `json:"title,omitempty"`
	TotalCoin   value.Money `json:"total_coin"`
	MemberCount int64       `json:"member_count"`
}

// PredictionRankResponse is the HTTP representation of a color's prediction ranking row.
// Accuracy is a percentage with two decimals.
type PredictionRankResponse struct {
	Rank     int     `json:"rank"`
	ID       string  `json:"id"`
	Title    string  `json:"title,omitempty"`
	Correct  int64   `json:"correct"`
	Wrong    int64   `json:"wrong"`
	Total    int64   `json:"total"`
	Accuracy float64 `json:"accuracy"`
}
