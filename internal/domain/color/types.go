package color

import "github.com/esc-chula/intania-888-backend/internal/value"

// Leaderboard contains a color's completed-match wins, draws, losses, and total.
type Leaderboard struct {
	ID         string
	Title      string
	Won        int64
	Drawn      int64
	Lost       int64
	TotalMatch int64
}

// Standing contains completed-match counts before the service derives losses.
type Standing struct {
	ID           string
	Title        string
	Won          int64
	Drawn        int64
	TotalMatches int64
}

// CoinStanding is a color's coin total before ranking. It sums the balances of
// USER accounts in every group that belongs to the color.
type CoinStanding struct {
	ID          string
	Title       string
	TotalCoin   value.Money
	MemberCount int64
}

// PredictionStanding counts decided bill-line predictions by a color's members.
type PredictionStanding struct {
	ID      string
	Title   string
	Correct int64
	Wrong   int64
}

// CoinRank is a ranked CoinStanding; Rank starts at 1.
type CoinRank struct {
	Rank int
	CoinStanding
}

// PredictionRank is a ranked PredictionStanding. Accuracy is the percentage of
// decided predictions that were correct, or 0 when none are decided.
type PredictionRank struct {
	Rank int
	PredictionStanding
	Total    int64
	Accuracy float64
}
