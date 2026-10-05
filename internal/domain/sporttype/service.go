package sporttype

import (
	"context"

	"go.uber.org/zap"

	"github.com/esc-chula/intania-888-backend/internal/catalogueid"
)

// Service provides the sport catalogue and validates management operations.
type Service struct {
	sportTypeRepo Repository
	log           *zap.Logger
}

// NewService constructs the sport catalogue service from persistence and logging.
func NewService(sportTypeRepo Repository, log *zap.Logger) *Service {
	if log == nil {
		log = zap.NewNop()
	}

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

// GetSportType returns one entry with a valid catalogue identifier.
func (s *Service) GetSportType(ctx context.Context, id string) (*SportType, error) {
	if !catalogueid.Valid(id) {
		return nil, ErrInvalidSportType
	}

	return s.sportTypeRepo.GetSportType(ctx, id)
}

// CreateSportType validates the immutable ID and normalizes the display title.
func (s *Service) CreateSportType(ctx context.Context, input SportType) (*SportType, error) {
	if !catalogueid.Valid(input.ID) {
		return nil, ErrInvalidSportType
	}
	title, err := normalizeTitle(input.Title)
	if err != nil {
		return nil, err
	}

	input.Title = title

	return s.sportTypeRepo.CreateSportType(ctx, input)
}

// UpdateSportType changes only the title of an existing entry.
func (s *Service) UpdateSportType(ctx context.Context, id, title string) (*SportType, error) {
	if !catalogueid.Valid(id) {
		return nil, ErrInvalidSportType
	}
	normalized, err := normalizeTitle(title)
	if err != nil {
		return nil, err
	}

	return s.sportTypeRepo.UpdateSportType(ctx, id, normalized)
}

// DeleteSportType removes an entry only when database constraints permit it.
func (s *Service) DeleteSportType(ctx context.Context, id string) error {
	if !catalogueid.Valid(id) {
		return ErrInvalidSportType
	}

	return s.sportTypeRepo.DeleteSportType(ctx, id)
}
