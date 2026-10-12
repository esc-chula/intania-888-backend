package stakemine

import (
	"time"

	"github.com/esc-chula/intania-888-backend/internal/value"
)

// CreateInput supplies a fixed-point stake and one of the configured risk levels.
type CreateInput struct {
	BetAmount value.Money
	RiskLevel string
}

// RevealInput selects a zero-based tile for an owned active game.
type RevealInput struct {
	Index int
}

// TileResult describes a visible tile; unrevealed active tiles use the hidden type.
type TileResult struct {
	Index    int
	Type     string // diamond, bomb, hidden
	Revealed bool
}

// GameResult is a game view with safe tile visibility and fixed-point amounts.
type GameResult struct {
	ID            string
	UserID        string
	BetAmount     value.Money
	RiskLevel     string
	Grid          []TileResult
	RevealedCount int
	CurrentPayout value.Money
	Multiplier    value.Rate
	Status        string
	CreatedAt     time.Time
	CompletedAt   *time.Time
}

// StatsResult separates realized totals from nullable active-game exposure.
type StatsResult struct {
	TotalGames          int
	GamesWon            int
	GamesLost           int
	GamesCashedOut      int
	TotalWagered        value.Money
	TotalWinnings       value.Money
	NetProfit           value.SignedMoney
	ActiveWagered       *value.Money
	ActiveCurrentPayout *value.Money
	WinRate             float64
}

// HistoryResult is one completed or active game in a paginated account history.
type HistoryResult struct {
	GameID        string
	BetAmount     value.Money
	RiskLevel     string
	Status        string
	FinalPayout   value.Money
	Multiplier    value.Rate
	RevealedCount int
	CreatedAt     time.Time
	CompletedAt   *time.Time
}

// Game is the stored game state exposed through repository ports.
// Grid serialization and database column details remain inside the adapter.
type Game struct {
	ID            string
	UserID        string
	BetAmount     int64
	RiskLevel     string
	Status        string
	RevealedCount int
	CurrentPayout int64
	Multiplier    int64
	Grid          []Tile
	// GridError carries a persistence decode failure until the use case has checked ownership and state.
	GridError   error
	CreatedAt   time.Time
	UpdatedAt   time.Time
	CompletedAt *time.Time
}

// History records one committed tile reveal in minor and micro units.
type History struct {
	ID          string
	GameID      string
	TileIndex   int
	TileType    string
	Multiplier  int64
	PayoutAtHit int64
	CreatedAt   time.Time
}
