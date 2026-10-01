package stakemine

import (
	"bytes"
	"encoding/json"
	"errors"
	"time"

	"github.com/esc-chula/intania-888-backend/internal/value"
)

// CreateGameRequest defines the HTTP request.
type CreateGameRequest struct {
	BetAmount value.Money `json:"bet_amount" validate:"required"`
	RiskLevel string      `json:"risk_level" validate:"required,oneof=low medium high"`
}

// RevealTileRequest defines the HTTP request.
type RevealTileRequest struct {
	Index    int `json:"index" validate:"required,min=0,max=15"`
	indexSet bool
}

// TileResponse defines the HTTP response.
type TileResponse struct {
	Index    int    `json:"index"`
	Type     string `json:"type"` // diamond, bomb, hidden
	Revealed bool   `json:"revealed"`
}

// GameResponse defines the HTTP response.
type GameResponse struct {
	ID            string         `json:"id"`
	UserID        string         `json:"user_id"`
	BetAmount     value.Money    `json:"bet_amount"`
	RiskLevel     string         `json:"risk_level"`
	Grid          []TileResponse `json:"grid"`
	RevealedCount int            `json:"revealed_count"`
	CurrentPayout value.Money    `json:"current_payout"`
	Multiplier    value.Rate     `json:"multiplier"`
	Status        string         `json:"status"`
	CreatedAt     time.Time      `json:"created_at"`
	CompletedAt   *time.Time     `json:"completed_at,omitempty"`
}

// StatsResponse defines the HTTP response.
type StatsResponse struct {
	TotalGames          int               `json:"total_games"`
	GamesWon            int               `json:"games_won"`
	GamesLost           int               `json:"games_lost"`
	GamesCashedOut      int               `json:"games_cashed_out"`
	TotalWagered        value.Money       `json:"total_wagered"`
	TotalWinnings       value.Money       `json:"total_winnings"`
	NetProfit           value.SignedMoney `json:"net_profit"`
	ActiveWagered       *value.Money      `json:"active_wagered"`
	ActiveCurrentPayout *value.Money      `json:"active_current_payout"`
	// WinRate is an approximate percentage, not an exact payout multiplier.
	WinRate float64 `json:"win_rate"`
}

// HistoryResponse defines the HTTP response.
type HistoryResponse struct {
	GameID        string      `json:"game_id"`
	BetAmount     value.Money `json:"bet_amount"`
	RiskLevel     string      `json:"risk_level"`
	Status        string      `json:"status"`
	FinalPayout   value.Money `json:"final_payout"`
	Multiplier    value.Rate  `json:"multiplier"`
	RevealedCount int         `json:"revealed_count"`
	CreatedAt     time.Time   `json:"created_at"`
	CompletedAt   *time.Time  `json:"completed_at,omitempty"`
}

// RevealTileResponse includes the game state and the tile reveal message.
type RevealTileResponse struct {
	Message string        `json:"message"`
	Game    *GameResponse `json:"game"`
}

// CashOutResponse includes the game state and cash-out confirmation.
type CashOutResponse struct {
	Message string        `json:"message"`
	Game    *GameResponse `json:"game"`
}

// HistoryListResponse carries the existing game history page.
type HistoryListResponse struct {
	Data   []HistoryResponse `json:"data"`
	Limit  int               `json:"limit"`
	Offset int               `json:"offset"`
}

// ValidateRequest checks the accepted HTTP bet and risk range.
func (r CreateGameRequest) ValidateRequest() map[string]string {
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

// UnmarshalJSON distinguishes a present zero index from a missing or null field.
func (r *RevealTileRequest) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return errors.New("request must be a JSON object")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	for field := range fields {
		if field != "index" {
			return errors.New("unknown field")
		}
	}
	var wire struct {
		Index *int `json:"index"`
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&wire); err != nil {
		return err
	}
	r.indexSet = wire.Index != nil
	if wire.Index != nil {
		r.Index = *wire.Index
	}

	return nil
}

// ValidateRequest checks presence and the zero-based tile range.
func (r RevealTileRequest) ValidateRequest() map[string]string {
	if !r.indexSet || r.Index < 0 || r.Index > 15 {
		return map[string]string{"index": "must be between 0 and 15"}
	}

	return nil
}
