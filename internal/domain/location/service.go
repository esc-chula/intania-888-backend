package location

import (
	"context"

	"go.uber.org/zap"

	"github.com/esc-chula/intania-888-backend/internal/catalogueid"
)

// Service provides the venue catalogue and validates management operations.
type Service struct {
	repo Repository
	log  *zap.Logger
}

// NewService constructs venue use cases from persistence and logging.
func NewService(repo Repository, log *zap.Logger) *Service {
	if log == nil {
		log = zap.NewNop()
	}

	return &Service{
		repo: repo,
		log:  log,
	}
}

// GetAllLocations returns every configured venue.
func (s *Service) GetAllLocations(ctx context.Context) ([]*Location, error) {
	locations, err := s.repo.GetAllLocations(ctx)
	if err != nil {
		return nil, err
	}

	s.log.Named("GetAllLocations").Info("Retrieved all locations successfully", zap.Int("count", len(locations)))

	return locations, nil
}

// GetLocation returns one venue by its stable ID.
func (s *Service) GetLocation(ctx context.Context, id string) (*Location, error) {
	if !catalogueid.Valid(id) {
		return nil, ErrInvalidLocation
	}

	return s.repo.GetLocation(ctx, id)
}

// CreateLocation validates an immutable ID and normalizes the display title.
func (s *Service) CreateLocation(ctx context.Context, input Location) (*Location, error) {
	if !catalogueid.Valid(input.ID) {
		return nil, ErrInvalidLocation
	}

	title, err := normalizeTitle(input.Title)
	if err != nil {
		return nil, err
	}
	input.Title = title

	return s.repo.CreateLocation(ctx, input)
}

// UpdateLocation changes only the title of an existing venue.
func (s *Service) UpdateLocation(ctx context.Context, id, title string) (*Location, error) {
	if !catalogueid.Valid(id) {
		return nil, ErrInvalidLocation
	}

	normalized, err := normalizeTitle(title)
	if err != nil {
		return nil, err
	}

	return s.repo.UpdateLocation(ctx, id, normalized)
}

// DeleteLocation removes a venue only when database constraints permit it.
func (s *Service) DeleteLocation(ctx context.Context, id string) error {
	if !catalogueid.Valid(id) {
		return ErrInvalidLocation
	}

	return s.repo.DeleteLocation(ctx, id)
}
