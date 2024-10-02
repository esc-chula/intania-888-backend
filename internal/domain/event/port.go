package event

import "github.com/esc-chula/intania-888-backend/internal/model"

type EventRepository interface {
	SetDailyRewardCache(key string, value interface{}, ttl int) error
	GetDailyRewardCache(key string, value interface{}) error
	GetReward(date string) (*model.DailyReward, error)
}

type EventService interface {
	RedeemDailyReward(req *model.UserDto) error
}
