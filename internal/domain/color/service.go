package color

import (
	"context"
	"math"
	"sort"

	"go.uber.org/zap"
)

// Service derives leaderboard loss counts from completed-match aggregate projections and
// ranks colors by team coins and by their members' correct bets.
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

// GetCoinRanking ranks colors by total coins, then by color ID.
func (s *Service) GetCoinRanking(ctx context.Context) ([]*CoinRank, error) {
	rows, err := s.colorRepo.CoinStandings(ctx)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if c := rows[i].TotalCoin.Compare(rows[j].TotalCoin); c != 0 {
			return c > 0
		}

		return rows[i].ID < rows[j].ID
	})

	ranks := make([]*CoinRank, len(rows))
	for i, row := range rows {
		ranks[i] = &CoinRank{Rank: i + 1, CoinStanding: *row}
	}

	return ranks, nil
}

// GetPredictionRanking ranks colors by correct predictions, then fewer wrong
// predictions, then color ID.
func (s *Service) GetPredictionRanking(ctx context.Context) ([]*PredictionRank, error) {
	rows, err := s.colorRepo.PredictionStandings(ctx)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Correct != rows[j].Correct {
			return rows[i].Correct > rows[j].Correct
		}
		if rows[i].Wrong != rows[j].Wrong {
			return rows[i].Wrong < rows[j].Wrong
		}

		return rows[i].ID < rows[j].ID
	})

	ranks := make([]*PredictionRank, len(rows))
	for i, row := range rows {
		total := row.Correct + row.Wrong
		accuracy := 0.0
		if total > 0 {
			accuracy = math.Round(float64(row.Correct)/float64(total)*10000) / 100
		}
		ranks[i] = &PredictionRank{Rank: i + 1, PredictionStanding: *row, Total: total, Accuracy: accuracy}
	}

	return ranks, nil
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
