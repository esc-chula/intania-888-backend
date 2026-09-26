package middleware

import (
	"errors"

	"github.com/esc-chula/intania-888-backend/internal/model"
	"github.com/esc-chula/intania-888-backend/internal/security"
	"github.com/esc-chula/intania-888-backend/pkg/cache"
	"github.com/esc-chula/intania-888-backend/pkg/config"
	"github.com/esc-chula/intania-888-backend/utils"
	"go.uber.org/zap"
)

type middlewareServiceImpl struct {
	repo   MiddlewareRepository
	cache  *cache.RedisClient
	policy security.AccessPolicyChecker
	log    *zap.Logger
	cfg    config.Config
}

func NewMiddlewareService(repo MiddlewareRepository, cache *cache.RedisClient, log *zap.Logger, cfg config.Config, policies ...security.AccessPolicyChecker) MiddlewareService {
	policyChecker := security.AccessPolicyChecker(security.DefaultPolicyChecker{})
	if len(policies) > 0 && policies[0] != nil {
		policyChecker = policies[0]
	}
	return &middlewareServiceImpl{
		repo:   repo,
		cache:  cache,
		policy: policyChecker,
		log:    log,
		cfg:    cfg,
	}
}

func (s *middlewareServiceImpl) VerifyToken(token string) (*utils.AccessTokenClaims, error) {
	claims, err := utils.JwtParseAccessToken(
		token,
		s.cfg.GetJwt().AccessTokenSecret,
		s.cfg.GetServer().Name,
		s.cfg.GetServer().Name,
	)
	if err != nil {
		s.log.Named("VerifyToken").Error("Parse access token", zap.Error(err))
		return nil, errors.New("invalid token")
	}

	var session model.SessionRecord
	if err := s.cache.GetValue(utils.ToSessionCacheKey(claims.SessionId), &session); err != nil {
		s.log.Named("VerifyToken").Error("Get active session", zap.Error(err))
		return nil, errors.New("inactive session")
	}
	if session.Id != claims.SessionId || session.UserId != claims.UserId {
		return nil, errors.New("inactive session")
	}

	return claims, nil
}

func (s *middlewareServiceImpl) GetMe(userId string) (*model.UserDto, error) {
	user, err := s.repo.GetById(userId)
	if err != nil {
		s.log.Named("GetMe").Error("Get user by id", zap.Error(err))
		return nil, err
	}

	return &model.UserDto{
		Id:            user.Id,
		Name:          user.Name,
		Email:         user.Email,
		RoleId:        user.RoleId,
		RemainingCoin: model.MustMoneyFromMinor(user.RemainingCoin),
		GroupId:       user.GroupId,
		NickName:      user.NickName,
	}, nil
}

func (s *middlewareServiceImpl) IsBlacklisted(email, userID string) (bool, error) {
	return s.policy.IsBlacklisted(email, userID)
}
