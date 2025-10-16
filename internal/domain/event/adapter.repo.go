package event

import (
	"errors"
	"math/rand/v2"
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

func (r *eventRepository) SetReward(reward *model.DailyReward) error {
	return r.db.Save(reward).Error
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

// StealPercentageFromRandomUsers steals a percentage from random users and transfers to the thief
func (r *eventRepository) StealPercentageFromRandomUsers(thiefUserId string, victimCount int, percentage float64) (float64, []model.VictimDetailDto, error) {
	var totalStolen float64
	var details []model.VictimDetailDto

	if percentage <= 0 {
		return 0, nil, errors.New("invalid percentage")
	}

	err := r.db.Transaction(func(tx *gorm.DB) error {
		// Lock thief row to prevent race conditions updating balance
		var thief model.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", thiefUserId).First(&thief).Error; err != nil {
			return err
		}

		// Fetch random victims
		var victims []model.User
		if err := tx.Where("id != ? AND remaining_coin >= ?", thiefUserId, 100.0).
			Order("RANDOM()").Limit(victimCount).Find(&victims).Error; err != nil {
			return err
		}
		if len(victims) == 0 {
			return errors.New("no eligible victims found")
		}

		// Iterate victims and transfer
		for _, v := range victims {
			steal := v.RemainingCoin * percentage
			if steal <= 0 {
				continue
			}

			// Lock victim row
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", v.Id).First(&v).Error; err != nil {
				return err
			}

			// Recalculate with current value (after lock)
			steal = v.RemainingCoin * percentage

			if err := tx.Model(&model.User{}).Where("id = ?", v.Id).
				Update("remaining_coin", gorm.Expr("remaining_coin - ?", steal)).Error; err != nil {
				return err
			}
			if err := tx.Model(&model.User{}).Where("id = ?", thiefUserId).
				Update("remaining_coin", gorm.Expr("remaining_coin + ?", steal)).Error; err != nil {
				return err
			}

			details = append(details, model.VictimDetailDto{
				UserId:       v.Id,
				Name:         v.Name,
				RoleId:       v.RoleId,
				GroupId:      v.GroupId,
				AmountStolen: steal,
			})
			totalStolen += steal
		}
		_ = rand.Float64() // keep rand imported for potential randomness extensions
		return nil
	})

	return totalStolen, details, err
}

// StealPercentageFromSpecificUser steals a percentage from a provided victim
// and transfers to the thief.
func (r *eventRepository) StealPercentageFromSpecificUser(thiefUserId string, victimUserId string, percentage float64) (float64, *model.VictimDetailDto, error) {
	var totalStolen float64
	var detail *model.VictimDetailDto

	if percentage <= 0 {
		return 0, nil, errors.New("invalid percentage")
	}
	if thiefUserId == victimUserId {
		return 0, nil, errors.New("cannot steal from yourself")
	}

	err := r.db.Transaction(func(tx *gorm.DB) error {
		// Lock thief and victim rows
		var thief model.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", thiefUserId).First(&thief).Error; err != nil {
			return err
		}
		var victim model.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", victimUserId).First(&victim).Error; err != nil {
			return err
		}
		if victim.RemainingCoin < 1 {
			return errors.New("victim has insufficient balance")
		}

		steal := victim.RemainingCoin * percentage
		if steal <= 0 {
			return errors.New("calculated steal is zero")
		}

		if err := tx.Model(&model.User{}).Where("id = ?", victim.Id).
			Update("remaining_coin", gorm.Expr("remaining_coin - ?", steal)).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.User{}).Where("id = ?", thief.Id).
			Update("remaining_coin", gorm.Expr("remaining_coin + ?", steal)).Error; err != nil {
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

// GetRandomEligibleUsers returns random users with balance >= 100, excluding the thief
func (r *eventRepository) GetRandomEligibleUsers(excludeUserId string, limit int) ([]model.User, error) {
	var users []model.User
	if err := r.db.Where("id != ? AND remaining_coin >= ?", excludeUserId, 100.0).Order("RANDOM()").Limit(limit).Find(&users).Error; err != nil {
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
