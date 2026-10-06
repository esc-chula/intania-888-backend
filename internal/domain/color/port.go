package color

import "context"

// ServicePort exposes the leaderboard use cases required by HTTP.
type ServicePort interface {
	// GetAllLeaderboards returns color standings, optionally restricted to a sport type.
	GetAllLeaderboards(context.Context, string) ([]*Leaderboard, error)
	// GetGroupStageTable returns standings with optional sport-type and group-stage filters.
	GetGroupStageTable(context.Context, string, string) ([]*Leaderboard, error)
	// GetCoinRanking returns colors ordered by team coins, highest first.
	GetCoinRanking(context.Context) ([]*CoinRank, error)
	// GetPredictionRanking returns colors ordered by their members' correct bets, highest first.
	GetPredictionRanking(context.Context) ([]*PredictionRank, error)
}

// Repository supplies leaderboard aggregate projections.
type Repository interface {
	// GetAllLeaderboards returns color standings, optionally restricted to a sport type.
	GetAllLeaderboards(context.Context, string) ([]*Standing, error)
	// GetGroupStageTable returns standings with optional sport-type and group-stage filters.
	GetGroupStageTable(context.Context, string, string) ([]*Standing, error)
	// CoinStandings sums the team coin ledger per color; colors with no events total zero.
	CoinStandings(context.Context) ([]*CoinStanding, error)
	// PredictionStandings sums the ledger's right and wrong bets per color. Bets on drawn or
	// undecided matches and bets in voided bills are not counted.
	PredictionStandings(context.Context) ([]*PredictionStanding, error)
}
