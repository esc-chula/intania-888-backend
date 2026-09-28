package sporttype

import (
	"context"

	"go.uber.org/zap"
)

// Service provides the sport catalogue to application consumers.
type Service struct {
	sportTypeRepo Repository
	log           *zap.Logger
}

// NewService constructs the sport catalogue service from persistence and logging.
func NewService(sportTypeRepo Repository, log *zap.Logger) *Service {
	return &Service{
		sportTypeRepo: sportTypeRepo,
		log:           log,
	}
}

// GetAllSportTypes returns every configured sport type and preserves repository failures.
func (s *Service) GetAllSportTypes(ctx context.Context) ([]*SportType, error) {
	sportTypes, err := s.sportTypeRepo.GetAllSportTypes(ctx)
	if err != nil {

		return nil, err
	}

	results := sportTypes
	s.log.Named("GetAllSportTypes").Info("Retrieved all sport types successful", zap.Int("count", len(results)))
	return results, nil
}
