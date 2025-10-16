package event

import "github.com/esc-chula/intania-888-backend/internal/model"

type EventRepository interface {
	SetDailyRewardCache(key string, value interface{}, ttl int) error
	GetDailyRewardCache(key string, value interface{}) error
	GetReward(date string) (*model.DailyReward, error)
	SetReward(reward *model.DailyReward) error

	CreateStealToken(token *model.StealToken) error
	GetStealTokenByToken(token string) (*model.StealToken, error)
	MarkTokenAsUsed(tokenId string) error
	DeleteExpiredTokens() error

	StealPercentageFromRandomUsers(thiefUserId string, victimCount int, percentage float64) (float64, []model.VictimDetailDto, error)
	StealPercentageFromSpecificUser(thiefUserId string, victimUserId string, percentage float64) (float64, *model.VictimDetailDto, error)
	GetRandomEligibleUsers(excludeUserId string, limit int) ([]model.User, error)
	GetUsersByIds(userIds []string) ([]model.User, error)
}

type EventService interface {
	RedeemDailyReward(req *model.UserDto) error
	SpinSlotMachine(req *model.UserDto, spendAmount float64) (map[string]interface{}, error)
	SetDailyReward(date string, amount float64) error

	// Use steal token
	UseStealToken(userId string, token string, victimIndex int) (*model.UseStealTokenResponseDto, error)
}
