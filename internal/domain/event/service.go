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

	// Check whether this user already redeemed today's reward.
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
	dailyReward := model.MustMoneyFromMinor(30000)

	// Load and update the user's balance.
	user, err := s.userRepo.GetById(req.Id)
	if err != nil {
		s.log.Named("RedeemDailyReward").Error("Get user by Id: ", zap.Error(err))
		return err
	}

	user.RemainingCoin += dailyReward.MinorUnits()

	err = s.userRepo.Update(user)
	if err != nil {
		s.log.Named("RedeemDailyReward").Error("Update user: ", zap.Error(err))
		return err
	}

	// Set the cache for daily reward redemption
	dailyRewardCache = model.DailyRewardCacheDto{
		UserId: req.Id,
		Reward: dailyReward,
	}

	if err := s.eventRepo.SetDailyRewardCache(key, dailyRewardCache, s.cfg.GetJwt().RefreshTokenExpiration); err != nil {
		s.log.Named("RedeemDailyReward").Error("Set daily reward cache: ", zap.Error(err))
		return err
	}

	return nil
}

func (s *eventService) SpinSlotMachine(req *model.UserDto, spendAmount model.Money) (map[string]interface{}, error) {
	// Remove expired tokens before starting a new spin.
	if err := s.eventRepo.DeleteExpiredTokens(); err != nil {
		s.log.Named("SpinSlotMachine").Warn("failed to cleanup expired tokens", zap.Error(err))
	}

	// Load the current balance for selecting the slot probability tier.
	user, err := s.userRepo.GetById(req.Id)
	if err != nil {
		return nil, err
	}

	spinProfile := *req
	spinProfile.RemainingCoin = model.MustMoneyFromMinor(user.RemainingCoin)

	// Spin the slots
	slot1 := utils.GetRandomSlot(&spinProfile)
	slot2 := utils.GetRandomSlot(&spinProfile)
	slot3 := utils.GetRandomSlot(&spinProfile)

	// Calculate reward based on new rules
	var reward model.Money
	var stealToken *model.StealToken
	var previews []model.CandidatePreviewDto

	multiply := func(micro int64) {
		reward, _ = spendAmount.Mul(model.MustRateFromMicro(micro))
	}

	switch {
	// 3 matching aliens -> issue steal token
	case slot1 == "👽" && slot2 == "👽" && slot3 == "👽":
		// pick 3 candidates and store their IDs in token
		candidates, err := s.eventRepo.GetRandomEligibleUsers(req.Id, 3)
		if err != nil || len(candidates) == 0 {
			s.log.Named("SpinSlotMachine").Error("No eligible candidates", zap.Error(err))
			multiply(4000000)
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
		stealToken = token
		previews = make([]model.CandidatePreviewDto, 0, len(candidates))

		for i, u := range candidates {
			previews = append(previews, model.CandidatePreviewDto{Index: i, Name: u.Name, RoleId: u.RoleId, GroupId: u.GroupId})
		}
	// 3 matching gold symbols
	case slot1 == "💰" && slot2 == "💰" && slot3 == "💰":
		multiply(10000000)

	// 3 matching fruit symbols
	case slot1 == slot2 && slot2 == slot3:
		multiply(4000000)

	// 2 gold + 1 different symbol
	case (slot1 == "💰" && slot2 == "💰" && slot3 != "💰") ||
		(slot1 == "💰" && slot3 == "💰" && slot2 != "💰") ||
		(slot2 == "💰" && slot3 == "💰" && slot1 != "💰"):
		multiply(3000000)

	// 1 gold + 2 matching symbols
	case (slot1 == "💰" && slot2 == slot3 && slot2 != "💰") ||
		(slot2 == "💰" && slot1 == slot3 && slot1 != "💰") ||
		(slot3 == "💰" && slot1 == slot2 && slot1 != "💰"):
		multiply(2000000)

	// 1 gold + 2 different symbols
	case (slot1 == "💰" && slot2 != "💰" && slot3 != "💰") ||
		(slot2 == "💰" && slot1 != "💰" && slot3 != "💰") ||
		(slot3 == "💰" && slot1 != "💰" && slot2 != "💰"):
		multiply(1500000)

	// 2 matching fruit symbols
	case slot1 == slot2 || slot1 == slot3 || slot2 == slot3:
		multiply(750000)

	default:
		reward = model.Money{}
	}

	// Commit the debit, reward, and optional token as one transaction.
	if err := s.eventRepo.CommitSlotSpin(req.Id, spendAmount, reward, stealToken); err != nil {
		return nil, err
	}

	// Return the generated slots and reward.
	result := map[string]interface{}{
		"slots":  []string{slot1, slot2, slot3},
		"reward": reward,
	}

	if stealToken != nil {
		result["reward"] = model.Money{}
		result["stealToken"] = model.StealTokenDto{
			Token:       stealToken.Token,
			ExpiresAt:   stealToken.ExpiresAt,
			VictimCount: 3,
			Message:     "👽 ALIEN POWER! Use this token to steal from other players!",
		}
		result["candidates"] = previews
	}

	return result, nil
}

func (s *eventService) UseStealToken(userId string, token string, victimIndex int) (*model.UseStealTokenResponseDto, error) {
	// Validate and consume the token, transfer the coins, and apply the minimum
	// payout in one transaction.
	result, err := s.eventRepo.ConsumeStealToken(userId, token, victimIndex)
	if err != nil {
		return nil, err
	}

	candidateMap := make(map[string]model.User, len(result.Candidates))
	for _, u := range result.Candidates {
		candidateMap[u.Id] = u
	}

	allCandidatesDto := make([]model.VictimDetailDto, 0, 3)

	for i, victimId := range result.CandidateIDs {
		victim, found := candidateMap[victimId]

		if !found {
			allCandidatesDto = append(allCandidatesDto, model.VictimDetailDto{
				Index:         i,
				UserId:        "",
				Name:          "[Deleted User]",
				RoleId:        "UNKNOWN",
				GroupId:       nil,
				BalanceBefore: model.Money{},
				AmountStolen:  model.Money{},
				WasChosen:     (victimId == result.ChosenVictimID),
			})
			continue
		}

		wasChosen := victimId == result.ChosenVictimID
		var amountStolen model.Money

		if wasChosen {
			amountStolen = result.StolenAmount
		}

		allCandidatesDto = append(allCandidatesDto, model.VictimDetailDto{
			Index:         i,
			UserId:        victim.Id,
			Name:          victim.Name,
			RoleId:        victim.RoleId,
			GroupId:       victim.GroupId,
			BalanceBefore: model.MustMoneyFromMinor(victim.RemainingCoin), // Balance BEFORE raid
			AmountStolen:  amountStolen,
			WasChosen:     wasChosen,
		})
	}

	chosenVictim, exists := candidateMap[result.ChosenVictimID]
	if !exists {
		return nil, errors.New("chosen victim no longer exists")
	}

	message := fmt.Sprintf("👽 You raided %s and stole %s coins!", chosenVictim.Name, result.StolenAmount.String())

	return &model.UseStealTokenResponseDto{
		TotalStolen:      result.StolenAmount,
		RaiderNewBalance: result.RaiderBalance,
		AllCandidates:    allCandidatesDto,
		Message:          message,
	}, nil
}

func (s *eventService) SetDailyReward(date string, amount model.Money) error {
	reward := &model.DailyReward{
		Date:   date,
		Reward: amount.MinorUnits(),
	}

	err := s.eventRepo.SetReward(reward)
	if err != nil {
		s.log.Named("SetDailyReward").Error("Failed to set daily reward", zap.Error(err))
		return err
	}

	s.log.Named("SetDailyReward").Info("Set daily reward successfully", zap.String("date", date), zap.Int64("amount_minor", amount.MinorUnits()))

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
