package match

import (
	"context"
	"time"

	"github.com/esc-chula/intania-888-backend/internal/domain/betting"

	"github.com/esc-chula/intania-888-backend/internal/value"
)

// Service coordinates match operations and atomic bill settlement.
type Service struct {
	repo         Repository
	transactions TransactionManager
	now          func() time.Time
	newID        func() string
	teamCoinsSvc TeamCoins
}

// NewService constructs match use cases with explicit repository, clock, and ID dependencies.
// teamCoinsSvc records the team coin ledger whenever a result is set.
func NewService(repo Repository, transactions TransactionManager, now func() time.Time, newID func() string, teamCoinsSvc TeamCoins) *Service {
	return &Service{
		repo:         repo,
		transactions: transactions,
		now:          now,
		newID:        newID,
		teamCoinsSvc: teamCoinsSvc,
	}
}

// CreateMatch creates a match from validated application details.
func (s *Service) CreateMatch(ctx context.Context, input *Input) error {
	if input == nil ||
		input.TeamAID == "" ||
		input.TeamBID == "" ||
		input.TypeID == "" ||
		input.LocationID == "" ||
		!input.EndTime.After(input.StartTime) {
		return ErrInvalidMatch
	}
	if input.ID == "" {
		input.ID = s.newID()
	}

	return s.repo.Create(ctx, snapshotFromInput(input))
}

// GetMatch loads a match and computes its authoritative rates.
func (s *Service) GetMatch(ctx context.Context, id string) (*Result, error) {
	snapshot, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	result := resultFromSnapshot(*snapshot)

	if snapshot.TeamAID != nil && snapshot.TeamBID != nil {
		teamACount, err := s.repo.CountBetsForTeam(ctx, id, *snapshot.TeamAID)
		if err != nil {
			return nil, err
		}

		teamBCount, err := s.repo.CountBetsForTeam(ctx, id, *snapshot.TeamBID)
		if err != nil {
			return nil, err
		}
		result.TeamARate, err = rateFor(teamACount, teamBCount, true)
		if err != nil {
			return nil, err
		}

		result.TeamBRate, err = rateFor(teamACount, teamBCount, false)
		if err != nil {
			return nil, err
		}
	}

	return result, nil
}

// GetAllMatches loads matching records without changing the existing query sequence.
func (s *Service) GetAllMatches(ctx context.Context, filter *Filter) ([]*Result, error) {
	snapshots, err := s.repo.GetAll(ctx, filter, s.now())
	if err != nil {
		return nil, err
	}

	results := make([]*Result, 0, len(snapshots))

	for _, snapshot := range snapshots {
		result, err := s.GetMatch(ctx, snapshot.ID)
		if err != nil {
			return nil, err
		}

		results = append(results, result)
	}

	return results, nil
}

// UpdateMatchScore updates scores without setting a terminal outcome.
func (s *Service) UpdateMatchScore(ctx context.Context, id string, input *ScoreInput) error {
	if input == nil || input.TeamAScore < 0 || input.TeamBScore < 0 {
		return ErrInvalidScore
	}
	snapshot, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	snapshot.TeamAScore = &input.TeamAScore
	snapshot.TeamBScore = &input.TeamBScore

	return s.repo.UpdateScore(ctx, snapshot)
}

// UpdateMatch updates match details using existing nonempty-field semantics.
func (s *Service) UpdateMatch(ctx context.Context, id string, input *Input) error {
	if input == nil {
		return ErrInvalidMatch
	}
	snapshot, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if input.TeamAID != "" {
		snapshot.TeamAID = &input.TeamAID
	}

	if input.TeamBID != "" {
		snapshot.TeamBID = &input.TeamBID
	}

	if input.TypeID != "" {
		snapshot.TypeID = input.TypeID
	}

	if input.LocationID != "" {
		snapshot.LocationID = input.LocationID
	}

	if !input.StartTime.IsZero() {
		snapshot.StartTime = input.StartTime
	}

	if !input.EndTime.IsZero() {
		snapshot.EndTime = input.EndTime
	}
	if !snapshot.EndTime.After(snapshot.StartTime) {
		return ErrInvalidMatch
	}

	return s.repo.UpdateMatch(ctx, snapshot, s.now())
}

// DeleteMatch deletes a match with its existing database constraints.
func (s *Service) DeleteMatch(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

// GetTime returns the current server time in UTC.
func (s *Service) GetTime() (string, error) {
	return s.now().UTC().Format(time.RFC3339), nil
}

func rateFor(a, b int64, forA bool) (value.Rate, error) {
	return betting.SeededRate(a, b, forA)
}
