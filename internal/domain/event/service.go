package event

import (
	"errors"
	"fmt"
	"time"

	"github.com/esc-chula/intania-888-backend/internal/domain/user"
	"github.com/esc-chula/intania-888-backend/internal/model"
	"github.com/esc-chula/intania-888-backend/pkg/config"
	"github.com/esc-chula/intania-888-backend/utils"
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

	// Set value of daily reward to 300 coins
	dailyReward := 300.00

	// Update the user's coin balance
	user, err := s.userRepo.GetById(req.Id)
	if err != nil {
		s.log.Named("RedeemDailyReward").Error("Get user by Id: ", zap.Error(err))
		return err
	}
	user.RemainingCoin += dailyReward

	err = s.userRepo.Update(user)
	if err != nil {
		s.log.Named("RedeemDailyReward").Error("Update user: ", zap.Error(err))
		return err
	}

	// Set the cache for daily reward redemption
	dailyRewardCache = model.DailyRewardCacheDto{UserId: req.Id, Reward: dailyReward}
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

	user.RemainingCoin -= spendAmount
	err = s.userRepo.Update(user)
	if err != nil {
		return nil, err
	}

	// Spin the slots
	slot1 := utils.GetRandomSlot(req)
	slot2 := utils.GetRandomSlot(req)
	slot3 := utils.GetRandomSlot(req)

	// Calculate reward based on new rules
	var reward float64
	switch {
	// 3 matching gold symbols
	case slot1 == "💰" && slot2 == "💰" && slot3 == "💰":
		reward = spendAmount * 10.0

	// 3 matching fruit symbols
	case slot1 == slot2 && slot2 == slot3:
		reward = spendAmount * 4.0

	// 2 gold + 1 different symbol
	case (slot1 == "💰" && slot2 == "💰" && slot3 != "💰") ||
		(slot1 == "💰" && slot3 == "💰" && slot2 != "💰") ||
		(slot2 == "💰" && slot3 == "💰" && slot1 != "💰"):
		reward = spendAmount * 3.0

	// 1 gold + 2 matching symbols
	case (slot1 == "💰" && slot2 == slot3 && slot2 != "💰") ||
		(slot2 == "💰" && slot1 == slot3 && slot1 != "💰") ||
		(slot3 == "💰" && slot1 == slot2 && slot1 != "💰"):
		reward = spendAmount * 2.0

	// 1 gold + 2 different symbols
	case (slot1 == "💰" && slot2 != "💰" && slot3 != "💰") ||
		(slot2 == "💰" && slot1 != "💰" && slot3 != "💰") ||
		(slot3 == "💰" && slot1 != "💰" && slot2 != "💰"):
		reward = spendAmount * 1.5

	// 2 matching fruit symbols
	case slot1 == slot2 || slot1 == slot3 || slot2 == slot3:
		reward = spendAmount * 0.75

	default:
		reward = 0
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

func (s *eventService) SetDailyReward(date string, amount float64) error {
	reward := &model.DailyReward{
		Date:   date,
		Reward: amount,
	}

	err := s.eventRepo.SetReward(reward)
	if err != nil {
		s.log.Named("SetDailyReward").Error("Failed to set daily reward", zap.Error(err))
		return err
	}

	s.log.Named("SetDailyReward").Info("Set daily reward successfully", zap.String("date", date), zap.Float64("amount", amount))
	return nil
}
