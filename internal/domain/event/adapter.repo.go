package event

import (
	"github.com/esc-chula/intania-888-backend/internal/model"
	"github.com/esc-chula/intania-888-backend/pkg/cache"
	"gorm.io/gorm"
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
