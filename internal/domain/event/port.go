package event

import (
	"errors"

	"github.com/esc-chula/intania-888-backend/internal/model"
)

var (
	ErrDailyRewardOverrideNotFound = errors.New("daily reward override not found")
	ErrDailyRewardAlreadyClaimed   = errors.New("daily reward already claimed")
	ErrInsufficientBalance         = errors.New("insufficient balance")
	ErrStealTokenInvalid           = errors.New("invalid or expired steal token")
	ErrStealTokenConflict          = errors.New("steal token already used")
	ErrStealTokenForbidden         = errors.New("steal token is not owned by user")
	ErrInvalidStealRequest         = errors.New("invalid steal request")
)

type EventRepository interface {
	SetDailyRewardCache(key string, value interface{}, ttl int) error
	GetDailyRewardCache(key string, value interface{}) error
	GetReward(date string) (*model.DailyReward, error)
	ListRewards() ([]model.DailyReward, error)
	SetReward(reward *model.DailyReward) error
	DeleteReward(date string) error
	RedeemDailyReward(userID string, date string, defaultReward model.Money) (model.Money, error)

	CreateStealToken(token *model.StealToken) error
	GetStealTokenByToken(token string) (*model.StealToken, error)
	MarkTokenAsUsed(tokenId string) error
	DeleteExpiredTokens() error
	CommitSlotSpin(userId string, spendAmount model.Money, reward model.Money, token *model.StealToken) error
	ConsumeStealToken(userId string, token string, victimIndex int) (*StealTokenUseResult, error)

	StealPercentageFromRandomUsers(
		thiefUserId string,
		victimCount int,
		percentage model.Rate,
	) (model.Money, []model.VictimDetailDto, error)
	StealPercentageFromSpecificUser(
		thiefUserId string,
		victimUserId string,
		percentage model.Rate,
	) (model.Money, *model.VictimDetailDto, error)
	GetRandomEligibleUsers(excludeUserId string, limit int) ([]model.User, error)
	GetUsersByIds(userIds []string) ([]model.User, error)
}

type StealTokenUseResult struct {
	CandidateIDs   []string
	Candidates     []model.User
	ChosenVictimID string
	StolenAmount   model.Money
	RaiderBalance  model.Money
}

type EventService interface {
	RedeemDailyReward(req *model.UserDto) error
	GetDailyRewardSchedule() (*model.DailyRewardScheduleResponse, error)
	SpinSlotMachine(
		req *model.UserDto,
		spendAmount model.Money,
	) (map[string]interface{}, error)
	SetDailyReward(date string, amount model.Money) error
	DeleteDailyReward(date string) error

	// Use steal token
	UseStealToken(userId string, token string, victimIndex int) (*model.UseStealTokenResponseDto, error)
}
