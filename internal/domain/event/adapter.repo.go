package event

import (
	"errors"
	"math/rand/v2"
	"sort"
	"time"

	"github.com/esc-chula/intania-888-backend/internal/model"
	"github.com/esc-chula/intania-888-backend/pkg/cache"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type eventRepository struct {
	db    *gorm.DB
	cache cache.RedisClient
}

const (
	minStealVictimBalanceMinor int64 = 100_00
	minStealAmountMinor        int64 = 50_00
	stealPercentageMicro       int64 = 200000
)

func NewEventRepository(db *gorm.DB, cache cache.RedisClient) EventRepository {
	return &eventRepository{
		db:    db,
		cache: cache,
	}
}

func (r *eventRepository) SetDailyRewardCache(key string, value interface{}, ttl int) error {
	return r.cache.SetValue(key, value, ttl)
}

func (r *eventRepository) GetDailyRewardCache(key string, value interface{}) error {
	return r.cache.GetValue(key, value)
}

func (r *eventRepository) GetReward(date string) (*model.DailyReward, error) {
	var reward model.DailyReward

	if err := r.db.First(&reward, "date = ?", date).Error; err != nil {
		return nil, err
	}

	return &reward, nil
}

func (r *eventRepository) ListRewards() ([]model.DailyReward, error) {
	var rewards []model.DailyReward
	if err := r.db.Find(&rewards).Error; err != nil {
		return nil, err
	}

	return rewards, nil
}

func (r *eventRepository) SetReward(reward *model.DailyReward) error {
	return r.db.Save(reward).Error
}

// RedeemDailyReward applies the configured reward and records the claim atomically.
func (r *eventRepository) RedeemDailyReward(userID string, date string, defaultReward model.Money) (model.Money, error) {
	var credited model.Money

	err := r.db.Transaction(func(tx *gorm.DB) error {
		rewardMinor := defaultReward.MinorUnits()

		var configured model.DailyReward
		err := tx.Where("date = ?", date).First(&configured).Error
		switch {
		case err == nil:
			rewardMinor = configured.Reward
		case errors.Is(err, gorm.ErrRecordNotFound):
			// Use the default reward when no date-specific configuration exists.
		default:
			return err
		}

		reward, err := model.NewMoneyFromMinor(rewardMinor)
		if err != nil {
			return err
		}

		var user model.User
		if err := tx.
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", userID).
			First(&user).
			Error; err != nil {
			return err
		}

		claim := &model.DailyRewardClaim{
			UserId: userID,
			Date:   date,
			Reward: reward.MinorUnits(),
		}
		insert := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(claim)
		if insert.Error != nil {
			return insert.Error
		}
		if insert.RowsAffected != 1 {
			return errors.New("already redeemed daily reward")
		}

		balance, err := model.NewMoneyFromMinor(user.RemainingCoin)
		if err != nil {
			return err
		}

		newBalance, err := balance.Add(reward)
		if err != nil {
			return err
		}

		update := tx.Model(&model.User{}).
			Where("id = ?", userID).
			Update("remaining_coin", newBalance.MinorUnits())
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}

		credited = reward
		return nil
	})

	return credited, err
}

// --- Steal token repositories ---

func (r *eventRepository) CreateStealToken(token *model.StealToken) error {
	return r.db.Create(token).Error
}

func (r *eventRepository) GetStealTokenByToken(token string) (*model.StealToken, error) {
	var t model.StealToken

	if err := r.db.Where("token = ?", token).First(&t).Error; err != nil {
		return nil, err
	}

	return &t, nil
}

func (r *eventRepository) MarkTokenAsUsed(tokenId string) error {
	return r.db.Model(&model.StealToken{}).Where("id = ?", tokenId).Update("is_used", true).Error
}

func (r *eventRepository) DeleteExpiredTokens() error {
	return r.db.Where("expires_at < ?", time.Now()).Delete(&model.StealToken{}).Error
}

// CommitSlotSpin applies the debit, reward, and optional token creation atomically.
func (r *eventRepository) CommitSlotSpin(userId string, spendAmount model.Money, reward model.Money, token *model.StealToken) error {
	if spendAmount.MinorUnits() <= 0 {
		return errors.New("invalid spend amount")
	}

	return r.db.Transaction(func(tx *gorm.DB) error {
		var user model.User
		if err := tx.
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", userId).
			First(&user).
			Error; err != nil {
			return err
		}

		balance, err := model.NewMoneyFromMinor(user.RemainingCoin)
		if err != nil {
			return err
		}

		remaining, err := balance.Sub(spendAmount)
		if err != nil {
			return errors.New("insufficient coins")
		}

		newBalance, err := remaining.Add(reward)
		if err != nil {
			return err
		}

		if err := tx.Model(&model.User{}).
			Where("id = ?", userId).
			Update("remaining_coin", newBalance.MinorUnits()).
			Error; err != nil {
			return err
		}

		if token != nil {
			if token.UserId != userId {
				return errors.New("slot token owner mismatch")
			}

			if err := tx.Create(token).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

// ConsumeStealToken atomically validates and consumes a token while transferring
// the resulting amount to the thief.
func (r *eventRepository) ConsumeStealToken(userId string, tokenValue string, victimIndex int) (*StealTokenUseResult, error) {
	result := &StealTokenUseResult{}

	err := r.db.Transaction(func(tx *gorm.DB) error {
		var token model.StealToken
		if err := tx.
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("token = ?", tokenValue).
			First(&token).
			Error; err != nil {
			return errors.New("invalid or expired token")
		}

		if token.UserId != userId {
			return errors.New("Idiot")
		}

		if token.IsUsed {
			return errors.New("token already used")
		}

		if time.Now().After(token.ExpiresAt) {
			return errors.New("token expired")
		}

		candidateIDs := splitCSV(token.AllowedVictimIds)
		if victimIndex < 0 || victimIndex >= len(candidateIDs) {
			return errors.New("Idiot")
		}

		chosenVictimID := candidateIDs[victimIndex]
		if chosenVictimID == userId {
			return errors.New("cannot steal from yourself")
		}

		// Lock users in a deterministic order so reciprocal raids cannot deadlock.
		lockIDs := []string{userId, chosenVictimID}
		sort.Strings(lockIDs)
		lockedUsers := make(map[string]model.User, len(lockIDs))
		for _, id := range lockIDs {
			var user model.User
			if err := tx.
				Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("id = ?", id).
				First(&user).
				Error; err != nil {
				return err
			}
			lockedUsers[id] = user
		}

		victim := lockedUsers[chosenVictimID]
		thief := lockedUsers[userId]
		if victim.RemainingCoin < minStealVictimBalanceMinor {
			return errors.New("chosen victim has insufficient balance")
		}

		percentage := model.MustRateFromMicro(stealPercentageMicro)
		stolen, err := model.MustMoneyFromMinor(victim.RemainingCoin).Mul(percentage)
		if err != nil {
			return err
		}

		if stolen.IsZero() {
			return errors.New("calculated steal is zero")
		}

		var candidates []model.User
		if err := tx.Where("id IN ?", candidateIDs).Find(&candidates).Error; err != nil {
			return err
		}

		credit := stolen
		if stolen.MinorUnits() < minStealAmountMinor {
			credit = model.MustMoneyFromMinor(minStealAmountMinor)
		}

		victimBalance, err := model.MustMoneyFromMinor(victim.RemainingCoin).Sub(stolen)
		if err != nil {
			return err
		}
		thiefBalance, err := model.MustMoneyFromMinor(thief.RemainingCoin).Add(credit)
		if err != nil {
			return err
		}

		if err := tx.Model(&model.User{}).
			Where("id = ?", victim.Id).
			Update("remaining_coin", victimBalance.MinorUnits()).
			Error; err != nil {
			return err
		}

		if err := tx.Model(&model.User{}).
			Where("id = ?", thief.Id).
			Update("remaining_coin", thiefBalance.MinorUnits()).
			Error; err != nil {
			return err
		}

		if update := tx.Model(&model.StealToken{}).
			Where("id = ? AND is_used = false", token.Id).
			Update("is_used", true); update.Error != nil {
			return update.Error
		} else if update.RowsAffected != 1 {
			return errors.New("token already used")
		}

		result.CandidateIDs = candidateIDs
		result.Candidates = candidates
		result.ChosenVictimID = chosenVictimID
		result.StolenAmount = credit
		result.RaiderBalance = thiefBalance
		return nil
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}

// StealPercentageFromRandomUsers steals a percentage from random users and transfers to the thief
func (r *eventRepository) StealPercentageFromRandomUsers(thiefUserId string, victimCount int, percentage model.Rate) (model.Money, []model.VictimDetailDto, error) {
	var totalStolen model.Money
	var details []model.VictimDetailDto

	if percentage.MicroUnits() <= 0 {
		return model.Money{}, nil, errors.New("invalid percentage")
	}

	err := r.db.Transaction(func(tx *gorm.DB) error {
		// Lock thief row to prevent race conditions updating balance
		var thief model.User
		if err := tx.
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", thiefUserId).
			First(&thief).
			Error; err != nil {
			return err
		}

		// Fetch random victims
		var victims []model.User

		if err := tx.
			Where("id != ? AND remaining_coin >= ?", thiefUserId, 100_00).
			Order("RANDOM()").Limit(victimCount).Find(&victims).Error; err != nil {
			return err
		}

		if len(victims) == 0 {
			return errors.New("no eligible victims found")
		}

		// Iterate victims and transfer
		for _, v := range victims {
			steal, err := model.MustMoneyFromMinor(v.RemainingCoin).Mul(percentage)
			if err != nil {
				return err
			}

			if steal.IsZero() {
				continue
			}

			// Lock victim row
			if err := tx.
				Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("id = ?", v.Id).
				First(&v).
				Error; err != nil {
				return err
			}

			// Recalculate with current value (after lock)
			steal, err = model.MustMoneyFromMinor(v.RemainingCoin).Mul(percentage)
			if err != nil {
				return err
			}

			if err := tx.Model(&model.User{}).Where("id = ?", v.Id).
				Update("remaining_coin", gorm.Expr("remaining_coin - ?", steal.MinorUnits())).Error; err != nil {
				return err
			}

			if err := tx.Model(&model.User{}).Where("id = ?", thiefUserId).
				Update("remaining_coin", gorm.Expr("remaining_coin + ?", steal.MinorUnits())).Error; err != nil {
				return err
			}

			details = append(details, model.VictimDetailDto{
				UserId:       v.Id,
				Name:         v.Name,
				RoleId:       v.RoleId,
				GroupId:      v.GroupId,
				AmountStolen: steal,
			})
			totalStolen, err = totalStolen.Add(steal)
			if err != nil {
				return err
			}
		}

		_ = rand.Float64() // keep rand imported for potential randomness extensions

		return nil
	})

	return totalStolen, details, err
}

// StealPercentageFromSpecificUser steals a percentage from a provided victim
// and transfers to the thief.
func (r *eventRepository) StealPercentageFromSpecificUser(thiefUserId string, victimUserId string, percentage model.Rate) (model.Money, *model.VictimDetailDto, error) {
	var totalStolen model.Money
	var detail *model.VictimDetailDto

	if percentage.MicroUnits() <= 0 {
		return model.Money{}, nil, errors.New("invalid percentage")
	}

	if thiefUserId == victimUserId {
		return model.Money{}, nil, errors.New("cannot steal from yourself")
	}

	err := r.db.Transaction(func(tx *gorm.DB) error {
		// Lock thief and victim rows
		var thief model.User
		if err := tx.
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", thiefUserId).
			First(&thief).
			Error; err != nil {
			return err
		}

		var victim model.User

		if err := tx.
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", victimUserId).
			First(&victim).
			Error; err != nil {
			return err
		}

		if victim.RemainingCoin < minStealVictimBalanceMinor {
			return errors.New("victim has insufficient balance")
		}

		steal, err := model.MustMoneyFromMinor(victim.RemainingCoin).Mul(percentage)
		if err != nil {
			return err
		}

		if steal.IsZero() {
			return errors.New("calculated steal is zero")
		}

		if err := tx.Model(&model.User{}).Where("id = ?", victim.Id).
			Update("remaining_coin", gorm.Expr("remaining_coin - ?", steal.MinorUnits())).Error; err != nil {
			return err
		}

		if err := tx.Model(&model.User{}).Where("id = ?", thief.Id).
			Update("remaining_coin", gorm.Expr("remaining_coin + ?", steal.MinorUnits())).Error; err != nil {
			return err
		}

		d := model.VictimDetailDto{
			UserId:       victim.Id,
			Name:         victim.Name,
			RoleId:       victim.RoleId,
			GroupId:      victim.GroupId,
			AmountStolen: steal,
		}

		detail = &d
		totalStolen = steal

		return nil
	})

	return totalStolen, detail, err
}

// GetRandomEligibleUsers returns random users with balance >= 100.00, excluding the thief
func (r *eventRepository) GetRandomEligibleUsers(excludeUserId string, limit int) ([]model.User, error) {
	var users []model.User

	if err := r.db.Where("id != ? AND remaining_coin >= ?", excludeUserId, minStealVictimBalanceMinor).Order("RANDOM()").Limit(limit).Find(&users).Error; err != nil {
		return nil, err
	}

	return users, nil
}

func (r *eventRepository) GetUsersByIds(userIds []string) ([]model.User, error) {
	var users []model.User

	if err := r.db.Where("id IN ?", userIds).Find(&users).Error; err != nil {
		return nil, err
	}

	return users, nil
}
