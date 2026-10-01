package color

import (
	"context"

	"go.uber.org/zap"
)

// Service derives leaderboard loss counts from completed-match aggregate projections.
type Service struct {
	colorRepo Repository
	log       *zap.Logger
}

// NewService constructs the color leaderboard service from aggregate persistence and logging.
func NewService(colorRepo Repository, log *zap.Logger) *Service {
	return &Service{
		colorRepo: colorRepo,
		log:       log,
	}
}

// GetAllLeaderboards returns standings across all colors, optionally filtered by sport type.
func (s *Service) GetAllLeaderboards(ctx context.Context, typeID string) ([]*Leaderboard, error) {
	colors, err := s.colorRepo.GetAllLeaderboards(ctx, typeID)
	if err != nil {
		return nil, err
	}

	results := leaderboards(colors)
	s.log.Named("GetAllLeaderboards").Info("Retrieved all leaderboards successful", zap.Int("count", len(results)))

	return results, nil
}

// GetGroupStageTable returns group-stage standings using the optional sport and group filters.
func (s *Service) GetGroupStageTable(ctx context.Context, typeID, groupID string) ([]*Leaderboard, error) {
	colors, err := s.colorRepo.GetGroupStageTable(ctx, typeID, groupID)
	if err != nil {
		return nil, err
	}

	results := leaderboards(colors)
	s.log.Named("GetGroupStageTable").Info("Retrieved group stage successful", zap.Int("count", len(results)))

	return results, nil
}

func leaderboards(rows []*Standing) []*Leaderboard {
	result := make([]*Leaderboard, len(rows))
	for i, row := range rows {
		result[i] = &Leaderboard{
			ID:         row.ID,
			Title:      row.Title,
			Won:        row.Won,
			Drawn:      row.Drawn,
			Lost:       row.TotalMatches - row.Won - row.Drawn,
			TotalMatch: row.TotalMatches,
		}
	}

	return result
}
