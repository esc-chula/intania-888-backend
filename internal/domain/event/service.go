package event

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/esc-chula/intania-888-backend/internal/domain/user"
	"github.com/esc-chula/intania-888-backend/internal/model"
	"github.com/esc-chula/intania-888-backend/pkg/config"
	"github.com/esc-chula/intania-888-backend/utils"
	"github.com/google/uuid"
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
	if err := s.eventRepo.DeleteExpiredTokens(); err != nil {
		s.log.Named("SpinSlotMachine").Warn("failed to cleanup expired tokens", zap.Error(err))
	}

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
	// 3 matching aliens -> issue steal token
	case slot1 == "👽" && slot2 == "👽" && slot3 == "👽":
		// pick 3 candidates and store their IDs in token
		candidates, err := s.eventRepo.GetRandomEligibleUsers(req.Id, 3)
		if err != nil || len(candidates) == 0 {
			s.log.Named("SpinSlotMachine").Error("No eligible candidates", zap.Error(err))
			reward = spendAmount * 4.0
			break
		}

		ids := make([]string, 0, len(candidates))
		for _, u := range candidates {
			ids = append(ids, u.Id)
		}

		token := &model.StealToken{
			Id:               uuid.NewString(),
			UserId:           req.Id,
			Token:            uuid.NewString(),
			IsUsed:           false,
			AllowedVictimIds: joinCSV(ids),
			ExpiresAt:        time.Now().Add(60 * time.Second),
		}
		if err := s.eventRepo.CreateStealToken(token); err != nil {
			s.log.Named("SpinSlotMachine").Error("Failed to create steal token", zap.Error(err))
			reward = spendAmount * 4.0
		} else {
			previews := make([]model.CandidatePreviewDto, 0, len(candidates))
			for i, u := range candidates {
				previews = append(previews, model.CandidatePreviewDto{Index: i, Name: u.Name, RoleId: u.RoleId, GroupId: u.GroupId})
			}

			return map[string]interface{}{
				"slots":  []string{slot1, slot2, slot3},
				"reward": 0.0,
				"stealToken": model.StealTokenDto{
					Token:       token.Token,
					ExpiresAt:   token.ExpiresAt,
					VictimCount: 3,
					Message:     "👽 ALIEN POWER! Use this token to steal from other players!",
				},
				"candidates": previews,
			}, nil
		}
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

	reward = roundToTwoDecimals(reward)

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

func (s *eventService) UseStealToken(userId string, token string, victimIndex int) (*model.UseStealTokenResponseDto, error) {
	stealToken, err := s.eventRepo.GetStealTokenByToken(token)
	if err != nil {
		return nil, errors.New("invalid or expired token")
	}
	if stealToken.UserId != userId {
		return nil, errors.New("Idiot")
	}
	if stealToken.IsUsed {
		return nil, errors.New("token already used")
	}
	if time.Now().After(stealToken.ExpiresAt) {
		return nil, errors.New("token expired")
	}

	candidateIds := splitCSV(stealToken.AllowedVictimIds)
	if victimIndex < 0 || victimIndex >= len(candidateIds) {
		return nil, errors.New("Idiot")
	}
	chosenVictimId := candidateIds[victimIndex]

	allCandidates, err := s.eventRepo.GetUsersByIds(candidateIds)
	if err != nil {
		return nil, errors.New("failed to fetch candidates")
	}

	candidateMap := make(map[string]model.User)
	for _, u := range allCandidates {
		candidateMap[u.Id] = u
	}

	chosenVictim, exists := candidateMap[chosenVictimId]
	minVictimBalance := 100.0

	if !exists {
		return nil, errors.New("chosen victim no longer exists")
	}
	if chosenVictim.RemainingCoin < minVictimBalance {
		return nil, errors.New("chosen victim has insufficient balance")
	}

	percentage := 0.20
	stolenAmount, _, err := s.eventRepo.StealPercentageFromSpecificUser(userId, chosenVictimId, percentage)
	if err != nil {
		return nil, fmt.Errorf("raid failed: %v", err)
	}

	minStealAmount := 50.0
	if stolenAmount > 0 && stolenAmount < minStealAmount {
		difference := minStealAmount - stolenAmount
		raider, err := s.userRepo.GetById(userId)
		if err == nil {
			raider.RemainingCoin += difference
			if err := s.userRepo.Update(raider); err != nil {
				s.log.Named("UseStealToken").Warn("failed to apply minimum bonus", zap.Error(err))
			} else {
				stolenAmount = minStealAmount
			}
		}
	}

	stolenAmount = roundToTwoDecimals(stolenAmount)

	if err := s.eventRepo.MarkTokenAsUsed(stealToken.Id); err != nil {
		s.log.Named("UseStealToken").Error("mark token used", zap.Error(err))
	}

	raider, err := s.userRepo.GetById(userId)
	if err != nil {
		s.log.Named("UseStealToken").Warn("failed to get raider balance", zap.Error(err))
	}

	allCandidatesDto := make([]model.VictimDetailDto, 0, 3)
	for i, victimId := range candidateIds {
		victim, found := candidateMap[victimId]

		if !found {
			allCandidatesDto = append(allCandidatesDto, model.VictimDetailDto{
				Index:         i,
				UserId:        "",
				Name:          "[Deleted User]",
				RoleId:        "UNKNOWN",
				GroupId:       nil,
				BalanceBefore: 0.0,
				AmountStolen:  0.0,
				WasChosen:     (victimId == chosenVictimId),
			})
			continue
		}

		wasChosen := (victimId == chosenVictimId)
		amountStolen := 0.0
		if wasChosen {
			amountStolen = stolenAmount
		}

		allCandidatesDto = append(allCandidatesDto, model.VictimDetailDto{
			Index:         i,
			UserId:        victim.Id,
			Name:          victim.Name,
			RoleId:        victim.RoleId,
			GroupId:       victim.GroupId,
			BalanceBefore: roundToTwoDecimals(victim.RemainingCoin), // Balance BEFORE raid
			AmountStolen:  amountStolen,
			WasChosen:     wasChosen,
		})
	}

	message := fmt.Sprintf("👽 You raided %s and stole %.2f coins!", chosenVictim.Name, stolenAmount)

	return &model.UseStealTokenResponseDto{
		TotalStolen:      stolenAmount,
		RaiderNewBalance: roundToTwoDecimals(raider.RemainingCoin),
		AllCandidates:    allCandidatesDto,
		Message:          message,
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

// joinCSV joins a slice of strings into a comma-separated string.
func joinCSV(ids []string) string {
	if len(ids) == 0 {
		return ""
	}
	return strings.Join(ids, ",")
}

// splitCSV splits a comma-separated string into a slice of strings.
func splitCSV(s string) []string {
	if s == "" {
		return []string{}
	}
	// Avoid empty elements from accidental double commas
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func roundToTwoDecimals(value float64) float64 {
	return float64(int(value*100+0.5)) / 100
}
