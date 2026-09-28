package color

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
