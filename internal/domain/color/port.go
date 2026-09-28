package color

import "context"

// ServicePort exposes the leaderboard use cases required by HTTP.
type ServicePort interface {
	// GetAllLeaderboards returns color standings, optionally restricted to a sport type.
	GetAllLeaderboards(context.Context, string) ([]*Leaderboard, error)
	// GetGroupStageTable returns standings with optional sport-type and group-stage filters.
	GetGroupStageTable(context.Context, string, string) ([]*Leaderboard, error)
}

// Repository supplies leaderboard aggregate projections.
type Repository interface {
	// GetAllLeaderboards returns color standings, optionally restricted to a sport type.
	GetAllLeaderboards(context.Context, string) ([]*Standing, error)
	// GetGroupStageTable returns standings with optional sport-type and group-stage filters.
	GetGroupStageTable(context.Context, string, string) ([]*Standing, error)
}
