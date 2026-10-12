package bill

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/esc-chula/intania-888-backend/internal/domain/betting"

	"github.com/esc-chula/intania-888-backend/internal/domain/match"
	"github.com/esc-chula/intania-888-backend/internal/domain/teamcoin"
	"github.com/esc-chula/intania-888-backend/internal/value"
)

// Service coordinates bill validation, rates, and atomic balance transitions.
type Service struct {
	repo         Repository
	transactions TransactionManager
	now          func() time.Time
	newID        func() string
	teamCoinsSvc TeamCoins
}

// NewService constructs bill use cases with explicit persistence, clock, and ID dependencies.
// teamCoinsSvc corrects the team coin ledger when a bill is voided.
func NewService(repo Repository, transactions TransactionManager, now func() time.Time, newID func() string, teamCoinsSvc TeamCoins) *Service {
	return &Service{
		repo:         repo,
		transactions: transactions,
		now:          now,
		newID:        newID,
		teamCoinsSvc: teamCoinsSvc,
	}
}

// CreateBill validates selections and atomically stores a bill and debits its stake.
func (s *Service) CreateBill(ctx context.Context, userID string, req *CreateInput) (*Result, error) {
	if req == nil || req.Total.IsZero() || len(req.Lines) == 0 {
		return nil, ErrInvalidBill
	}

	lines := append([]Selection(nil), req.Lines...)
	sort.Slice(lines, func(i, j int) bool {
		return lines[i].MatchID < lines[j].MatchID
	})
	for i, line := range lines {
		matchIDEmpty := strings.TrimSpace(line.MatchID) == ""
		bettingOnEmpty := strings.TrimSpace(line.BettingOn) == ""
		duplicateMatch := i > 0 && line.MatchID == lines[i-1].MatchID
		if matchIDEmpty || bettingOnEmpty || duplicateMatch {
			return nil, ErrInvalidBill
		}
	}

	var made Result

	err := s.transactions.WithinTransaction(ctx, func(tx TransactionRepository, _ teamcoin.Repository) error {
		if err := tx.AcquireLifecycleLock(ctx); err != nil {
			return err
		}

		ids := make([]string, len(lines))
		for i := range lines {
			ids[i] = lines[i].MatchID
		}
		matches, err := tx.LockMatches(ctx, ids)
		if err != nil {
			return err
		}
		if len(matches) != len(lines) {
			return ErrMatchNotFound
		}

		byID := make(map[string]match.Snapshot, len(matches))
		for _, item := range matches {
			byID[item.ID] = item
		}

		made = Result{
			ID:        s.newID(),
			Total:     req.Total,
			UserID:    userID,
			Status:    StatusPending,
			CreatedAt: s.now(),
			UpdatedAt: s.now(),
		}

		rates := make([]value.Rate, 0, len(lines))

		now := s.now()
		for _, input := range lines {
			item := byID[input.MatchID]
			missingTeams := item.TeamAID == nil || item.TeamBID == nil
			invalidSelection := !missingTeams && input.BettingOn != *item.TeamAID && input.BettingOn != *item.TeamBID
			started := !now.Before(item.StartTime)
			if missingTeams || invalidSelection || started || item.WinnerID != nil || item.IsDraw {
				return ErrInvalidBill
			}
			counts, err := tx.CountBets(ctx, item.ID)
			if err != nil {
				return err
			}
			var a, b int64
			for _, count := range counts {
				if count.BettingOn == *item.TeamAID {
					a = count.Count
				}
				if count.BettingOn == *item.TeamBID {
					b = count.Count
				}
			}
			rate, err := seededRate(a, b, input.BettingOn == *item.TeamAID)
			if err != nil {
				return err
			}
			rates = append(rates, rate)
			made.Lines = append(made.Lines, Line{
				BillID:    made.ID,
				MatchID:   item.ID,
				BettingOn: input.BettingOn,
				Rate:      rate,
				Match:     item,
			})
		}
		if _, err := value.AccumulatorPayout(req.Total, rates); err != nil {
			return err
		}

		balance, err := tx.LockBalance(ctx, userID)
		if err != nil {
			return err
		}
		if balance.Lesser(req.Total) {
			return ErrInsufficientBalance
		}
		voteColorID, err := tx.LockVoteColor(ctx, userID)
		if err != nil {
			return err
		}
		for i := range made.Lines {
			made.Lines[i].VoteColorID = voteColorID
		}
		if err := tx.CreateBill(ctx, &made); err != nil {
			return err
		}

		return tx.DebitBalance(ctx, userID, req.Total)
	})
	if err != nil {
		return nil, err
	}

	return &made, nil
}

func seededRate(a, b int64, forA bool) (value.Rate, error) {
	return betting.SeededRate(a, b, forA)
}

// GetBill loads the bill belonging to the supplied user.
func (s *Service) GetBill(ctx context.Context, id, userID string) (*Result, error) {
	return s.repo.GetByID(ctx, id, userID)
}

// GetAllBills loads the user's bills in the repository's existing order.
func (s *Service) GetAllBills(ctx context.Context, userID string) ([]*Result, error) {
	return s.repo.GetAll(ctx, userID)
}

// GetAllBillsAdmin loads all bills for an administrator.
func (s *Service) GetAllBillsAdmin(ctx context.Context) ([]*Result, error) {
	return s.repo.GetAllAdmin(ctx)
}
