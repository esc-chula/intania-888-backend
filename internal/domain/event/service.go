package event

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/esc-chula/intania-888-backend/internal/identity"
	"github.com/esc-chula/intania-888-backend/internal/value"
)

var bangkokLocation = time.FixedZone("Asia/Bangkok", int((7*time.Hour)/time.Second))

const (
	minStealVictimBalanceMinor int64 = 100_00
	minStealAmountMinor        int64 = 50_00
	stealPercentageMicro       int64 = 200000
)

// Service applies event rewards and raid rules through atomic repository callbacks.
type Service struct {
	repo          Repository
	log           *zap.Logger
	now           func() time.Time
	drawSlot      func(identity.Profile) string
	defaultReward value.Money
}

// NewService constructs the event service with the configured default daily reward.
func NewService(repo Repository, defaultReward value.Money, log *zap.Logger) *Service {
	if log == nil {
		log = zap.NewNop()
	}

	return &Service{
		repo:          repo,
		defaultReward: defaultReward,
		log:           log,
		now:           time.Now,
		drawSlot:      getRandomSlot,
	}
}

// RedeemDailyReward credits one claim per actor and Bangkok calendar date atomically.
func (s *Service) RedeemDailyReward(ctx context.Context, userID string) error {
	date := s.now().In(bangkokLocation).Format("02-01-2006")
	err := s.repo.WithinTransaction(ctx, func(tx TransactionRepository) error {
		reward := s.defaultReward
		configured, err := tx.GetReward(ctx, date)
		switch {
		case err == nil:
			reward, err = value.NewMoneyFromMinor(configured.Reward)
			if err != nil {
				return err
			}
		case errors.Is(err, ErrDailyRewardOverrideNotFound):
			// The configured default applies when no date-specific override exists.
		default:
			return err
		}

		actor, err := tx.LockUser(ctx, userID)
		if err != nil {
			return err
		}
		created, err := tx.CreateDailyClaim(ctx, userID, date, reward)
		if err != nil {
			return err
		}
		if !created {
			return ErrDailyRewardAlreadyClaimed
		}

		balance, err := value.NewMoneyFromMinor(actor.RemainingCoin)
		if err != nil {
			return err
		}
		newBalance, err := balance.Add(reward)
		if err != nil {
			return err
		}

		return tx.SetUserBalance(ctx, userID, newBalance)
	})
	if err != nil {
		return err
	}
	s.log.Named("RedeemDailyReward").Info("Daily reward redeemed", zap.String("user_id", userID), zap.String("date", date))

	return nil
}

// GetDailyRewardSchedule returns the default and chronologically ordered overrides.
func (s *Service) GetDailyRewardSchedule(ctx context.Context) (*DailyRewardSchedule, error) {
	rewards, err := s.repo.ListRewards(ctx)
	if err != nil {
		return nil, err
	}
	response := &DailyRewardSchedule{
		DefaultAmount: s.defaultReward,
		Overrides:     make([]DailyRewardScheduleItem, 0, len(rewards)),
	}
	for _, reward := range rewards {
		amount, err := value.NewMoneyFromMinor(reward.Reward)
		if err != nil {
			return nil, fmt.Errorf("invalid daily reward amount for %q: %w", reward.Date, err)
		}
		response.Overrides = append(response.Overrides, DailyRewardScheduleItem{
			Date:   reward.Date,
			Amount: amount,
		})
	}
	sort.SliceStable(response.Overrides, func(i, j int) bool {
		left, leftErr := time.Parse("02-01-2006", response.Overrides[i].Date)
		right, rightErr := time.Parse("02-01-2006", response.Overrides[j].Date)
		switch {
		case leftErr == nil && rightErr == nil:
			if left.Equal(right) {
				return response.Overrides[i].Date < response.Overrides[j].Date
			}

			return left.Before(right)
		case leftErr == nil:
			return true
		case rightErr == nil:
			return false
		default:
			return response.Overrides[i].Date < response.Overrides[j].Date
		}
	})

	return response, nil
}

// SpinSlotMachine selects the existing slot outcomes and atomically applies the debit and reward.
func (s *Service) SpinSlotMachine(ctx context.Context, actor identity.Profile, spendAmount value.Money) (*SpinResult, error) {
	if err := s.repo.DeleteExpiredTokens(ctx, s.now()); err != nil {
		s.log.Named("SpinSlotMachine").Warn("failed to cleanup expired tokens", zap.Error(err))
	}
	current, err := s.repo.GetUser(ctx, actor.ID)
	if err != nil {
		return nil, err
	}
	actor.RemainingCoin, err = value.NewMoneyFromMinor(current.RemainingCoin)
	if err != nil {
		return nil, err
	}

	slot1 := s.drawSlot(actor)
	slot2 := s.drawSlot(actor)
	slot3 := s.drawSlot(actor)
	var reward value.Money
	var stealToken *StealToken
	var previews []CandidatePreview
	multiply := func(micro int64) {
		//nolint:errcheck // NORM-003 preserves the inherited zero reward on multiplication overflow; change separately.
		reward, _ = spendAmount.Mul(value.MustRateFromMicro(micro))
	}

	switch {
	// 3 matching aliens -> issue steal token
	case slot1 == "👽" && slot2 == "👽" && slot3 == "👽":
		// pick 3 candidates and store their IDs in token
		candidates, err := s.repo.GetRandomEligibleUsers(ctx, actor.ID, minStealVictimBalanceMinor, 3)
		if err != nil || len(candidates) == 0 {
			s.log.Named("SpinSlotMachine").Error("No eligible candidates", zap.Error(err))
			multiply(4000000)
			break
		}

		ids := make([]string, 0, len(candidates))
		for _, u := range candidates {
			ids = append(ids, u.ID)
		}

		token := &StealToken{
			ID:               uuid.NewString(),
			UserID:           actor.ID,
			Token:            uuid.NewString(),
			IsUsed:           false,
			AllowedVictimIDs: ids,
			ExpiresAt:        s.now().Add(60 * time.Second),
		}
		stealToken = token
		previews = make([]CandidatePreview, 0, len(candidates))

		for i, u := range candidates {
			previews = append(previews, CandidatePreview{
				Index:   i,
				Name:    u.Name,
				RoleID:  u.RoleID,
				GroupID: u.GroupID,
			})
		}
	// 3 matching gold symbols
	case slot1 == "💰" && slot2 == "💰" && slot3 == "💰":
		multiply(10000000)

	// 3 matching fruit symbols
	case slot1 == slot2 && slot2 == slot3:
		multiply(4000000)

	// 2 gold + 1 different symbol
	case (slot1 == "💰" && slot2 == "💰" && slot3 != "💰") ||
		(slot1 == "💰" && slot3 == "💰" && slot2 != "💰") ||
		(slot2 == "💰" && slot3 == "💰" && slot1 != "💰"):
		multiply(3000000)

	// 1 gold + 2 matching symbols
	case (slot1 == "💰" && slot2 == slot3 && slot2 != "💰") ||
		(slot2 == "💰" && slot1 == slot3 && slot1 != "💰") ||
		(slot3 == "💰" && slot1 == slot2 && slot1 != "💰"):
		multiply(2000000)

	// 1 gold + 2 different symbols
	case (slot1 == "💰" && slot2 != "💰" && slot3 != "💰") ||
		(slot2 == "💰" && slot1 != "💰" && slot3 != "💰") ||
		(slot3 == "💰" && slot1 != "💰" && slot2 != "💰"):
		multiply(1500000)

	// 2 matching fruit symbols
	case slot1 == slot2 || slot1 == slot3 || slot2 == slot3:
		multiply(750000)

	default:
		reward = value.Money{}
	}

	if err := s.CommitSlotSpin(ctx, actor.ID, spendAmount, reward, stealToken); err != nil {
		return nil, err
	}
	result := &SpinResult{
		Slots:  []string{slot1, slot2, slot3},
		Reward: reward,
	}
	if stealToken != nil {
		result.Reward = value.Money{}
		result.StealToken = &TokenReward{
			Token:       stealToken.Token,
			ExpiresAt:   stealToken.ExpiresAt,
			VictimCount: 3,
			Message:     "👽 ALIEN POWER! Use this token to steal from other players!",
		}
		result.Candidates = previews
	}

	return result, nil
}

// CommitSlotSpin commits a calculated spend, reward, and optional token together.
// Any write failure rolls back both the balance change and the token insertion.
func (s *Service) CommitSlotSpin(ctx context.Context, userID string, spendAmount, reward value.Money, token *StealToken) error {
	if spendAmount.IsZero() {
		return errors.New("invalid spend amount")
	}

	return s.repo.WithinTransaction(ctx, func(tx TransactionRepository) error {
		actor, err := tx.LockUser(ctx, userID)
		if err != nil {
			return err
		}
		balance, err := value.NewMoneyFromMinor(actor.RemainingCoin)
		if err != nil {
			return err
		}
		remaining, err := balance.Sub(spendAmount)
		if err != nil {
			return ErrInsufficientBalance
		}
		newBalance, err := remaining.Add(reward)
		if err != nil {
			return err
		}
		if err := tx.SetUserBalance(ctx, userID, newBalance); err != nil {
			return err
		}
		if token != nil {
			if token.UserID != userID {
				return errors.New("slot token owner mismatch")
			}
			if err := tx.CreateStealToken(ctx, *token); err != nil {
				return err
			}
		}

		return nil
	})
}

// SetDailyReward creates or replaces the configured override for one date.
func (s *Service) SetDailyReward(ctx context.Context, date string, amount value.Money) error {
	if err := s.repo.SetReward(ctx, DailyReward{
		Date:   date,
		Reward: amount.MinorUnits(),
	}); err != nil {
		return err
	}
	s.log.Named("SetDailyReward").Info(
		"Set daily reward successfully",
		zap.String("date", date),
		zap.Int64("amount_minor", amount.MinorUnits()),
	)

	return nil
}

// DeleteDailyReward removes an override so the configured default applies again.
func (s *Service) DeleteDailyReward(ctx context.Context, date string) error {
	if err := s.repo.DeleteReward(ctx, date); err != nil {
		return err
	}
	s.log.Named("DeleteDailyReward").Info("Deleted daily reward override", zap.String("date", date))

	return nil
}
