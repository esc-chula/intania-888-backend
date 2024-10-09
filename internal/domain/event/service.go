package event

import (
	"errors"
	"fmt"
	"time"

	"github.com/esc-chula/intania-888-backend/internal/domain/user"
	"github.com/esc-chula/intania-888-backend/internal/model"
	"github.com/esc-chula/intania-888-backend/pkg/config"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type eventService struct {
	eventRepo EventRepository
	userRepo  user.UserRepository
	cfg       config.Config
	log       *zap.Logger
}

func NewEventService(eventRepo EventRepository, userRepo user.UserRepository, cfg config.Config, log *zap.Logger) EventService {
	return &eventService{
		eventRepo: eventRepo,
		userRepo:  userRepo,
		cfg:       cfg,
		log:       log,
	}
}

func (s *eventService) RedeemDailyReward(req *model.UserDto) error {
	// is requested user has been already redeemed daily reward or not ?
	currentTime := time.Now()
	date := currentTime.Format("02-01-2006")
	key := fmt.Sprintf("%v/%v", date, req.Id)

	var dailyRewardCache model.DailyRewardCacheDto

	// Check if user has already redeemed the reward for today
	err := s.eventRepo.GetDailyRewardCache(key, &dailyRewardCache)
	if err != nil {
		if err == redis.Nil {
			// First-time redemption for today: proceed with reward
			s.log.Named("RedeemDailyReward").Info("First-time redemption for today", zap.String("user_id", req.Id))
		} else {
			// Handle other Redis errors
			s.log.Named("RedeemDailyReward").Error("Get daily reward cache: ", zap.Error(err))
			return err
		}
	} else {
		// User has already redeemed the reward today
		s.log.Named("RedeemDailyReward").Info("Already redeemed daily reward", zap.String("user_id", req.Id))
		return errors.New("already redeemed daily reward")
	}

	// Fetch today's reward from the database
	todayReward, err := s.eventRepo.GetReward(date)
	if err != nil {
		s.log.Named("RedeemDailyReward").Error("Get daily reward: ", zap.Error(err))
		return err
	}

	// Update the user's coin balance
	user, err := s.userRepo.GetById(req.Id)
	if err != nil {
		s.log.Named("RedeemDailyReward").Error("Get user by Id: ", zap.Error(err))
		return err
	}
	user.RemainingCoin += todayReward.Reward

	err = s.userRepo.Update(user)
	if err != nil {
		s.log.Named("RedeemDailyReward").Error("Update user: ", zap.Error(err))
		return err
	}

	// Set the cache for daily reward redemption
	dailyRewardCache = model.DailyRewardCacheDto{UserId: req.Id, Reward: todayReward.Reward}
	if err := s.eventRepo.SetDailyRewardCache(key, dailyRewardCache, s.cfg.GetJwt().RefreshTokenExpiration); err != nil {
		s.log.Named("RedeemDailyReward").Error("Set daily reward cache: ", zap.Error(err))
		return err
	}

	return nil
}

func (s *eventService) SpinSlotMachine(req *model.UserDto, spendAmount float64) (map[string]interface{}, error) {
	// Check if the user has enough coins
	user, err := s.userRepo.GetById(req.Id)
	if err != nil {
		return nil, err
	}

	if user.RemainingCoin < spendAmount {
		return nil, errors.New("insufficient coins")
	}

	// Deduct the chosen amount of coins for the spin
	user.RemainingCoin -= spendAmount
	err = s.userRepo.Update(user)
	if err != nil {
		return nil, err
	}

	// Spin the slots
	slot1 := utils.getRandomSlot()
	slot2 := utils.getRandomSlot()
	slot3 := utils.getRandomSlot()

	// Calculate reward (scaled by spending amount)
	var reward int
	switch {
	case slot1 == "💰" && slot2 == "💰" && slot3 == "💰":
		reward = 1000 * (spendAmount / 100) // 3 golds
	case (slot1 == slot2 && slot1 != "💰" && slot3 == "💰") || (slot2 == slot3 && slot2 != "💰" && slot1 == "💰") || (slot1 == slot3 && slot1 != "💰" && slot2 == "💰"):
		reward = 200 * (spendAmount / 100) // 1 gold and 2 matching fruits
	case (slot1 == "💰" && slot2 == "💰") || (slot2 == "💰" && slot3 == "💰") || (slot1 == "💰" && slot3 == "💰"):
		reward = 500 * (spendAmount / 100) // 2 golds
	case slot1 == "💰" || slot2 == "💰" || slot3 == "💰":
		reward = 100 * (spendAmount / 100) // 1 gold
	default:
		reward = 0 // No reward
	}

	// Add reward to user's balance
	user.RemainingCoin += reward
	err = s.userRepo.Update(user)
	if err != nil {
		return nil, err
	}

	// Return result to frontend
	return map[string]interface{}{
		"slots":  []string{slot1, slot2, slot3},
		"reward": reward,
	}, nil
}
