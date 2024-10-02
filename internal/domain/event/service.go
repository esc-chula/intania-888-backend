package event

import (
	"errors"
	"fmt"
	"time"

	"github.com/esc-chula/intania-888-backend/internal/domain/user"
	"github.com/esc-chula/intania-888-backend/internal/model"
	"github.com/esc-chula/intania-888-backend/pkg/config"
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
	var emptyCache model.DailyRewardCacheDto
	if err := s.eventRepo.GetDailyRewardCache(key, &dailyRewardCache); err != nil {
		s.log.Named("RedeemDailyReward").Error("Get daily reward cache: ", zap.Error(err))
		return err
	} else if dailyRewardCache != emptyCache {
		s.log.Named("RedeemDailyReward").Info("Compare with empty cache: already redeem daily reward")
		return errors.New("already redeem daily reward")
	}

	// if not redeem yet, find today reward cache
	todayReward, err := s.eventRepo.GetReward(date)
	if err != nil {
		s.log.Named("RedeemDailyReward").Error("Get daily reward: ", zap.Error(err))
		return err
	}

	// update user balance
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

	// set cache
	dailyRewardCache = model.DailyRewardCacheDto{UserId: req.Id, Reward: todayReward.Reward}
	if err := s.eventRepo.SetDailyRewardCache(key, dailyRewardCache, s.cfg.GetJwt().RefreshTokenExpiration); err != nil {
		s.log.Named("RedeemDailyReward").Error("Set daily reward cache: ", zap.Error(err))
		return err
	}

	return nil
}
