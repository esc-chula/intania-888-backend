package color

import "context"

// ServicePort exposes the leaderboard use cases required by HTTP.
type ServicePort interface {
	// GetAllLeaderboards returns color standings, optionally restricted to a sport type.
	GetAllLeaderboards(context.Context, string) ([]*Leaderboard, error)
	// GetGroupStageTable returns standings with optional sport-type and group-stage filters.
	GetGroupStageTable(context.Context, string, string) ([]*Leaderboard, error)
	// GetCoinRanking returns colors ordered by total member coins, highest first.
	GetCoinRanking(context.Context) ([]*CoinRank, error)
	// GetPredictionRanking returns colors ordered by correct member predictions, highest first.
	GetPredictionRanking(context.Context) ([]*PredictionRank, error)
}

// Repository supplies leaderboard aggregate projections.
type Repository interface {
	// GetAllLeaderboards returns color standings, optionally restricted to a sport type.
	GetAllLeaderboards(context.Context, string) ([]*Standing, error)
	// GetGroupStageTable returns standings with optional sport-type and group-stage filters.
	GetGroupStageTable(context.Context, string, string) ([]*Standing, error)
	// CoinStandings sums the coins of USER accounts per color; colors without members total zero.
	CoinStandings(context.Context) ([]*CoinStanding, error)
	// PredictionStandings counts bill lines of WON and LOST bills whose match has a winner.
	// Draws, undecided matches, pending bills, and voided bills are not counted.
	PredictionStandings(context.Context) ([]*PredictionStanding, error)
}
