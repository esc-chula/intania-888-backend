package middleware

import (
	"encoding/base64"
	"errors"
	"time"

	"github.com/esc-chula/intania-888-backend/internal/model"
	"github.com/esc-chula/intania-888-backend/internal/security"
	"github.com/esc-chula/intania-888-backend/pkg/cache"
	"github.com/esc-chula/intania-888-backend/pkg/config"
	"github.com/esc-chula/intania-888-backend/utils"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

var (
	ErrSessionMissing  = errors.New("session missing")
	ErrExternalMissing = errors.New("external token missing")
)

type middlewareServiceImpl struct {
	repo   MiddlewareRepository
	cache  *cache.RedisClient
	policy security.AccessPolicyChecker
	log    *zap.Logger
	cfg    config.Config
}

func NewMiddlewareService(
	repo MiddlewareRepository,
	c *cache.RedisClient,
	log *zap.Logger,
	cfg config.Config,
	policies ...security.AccessPolicyChecker,
) MiddlewareService {
	p := security.AccessPolicyChecker(security.DefaultPolicyChecker{})
	if len(policies) > 0 && policies[0] != nil {
		p = policies[0]
	}

	return &middlewareServiceImpl{
		repo:   repo,
		cache:  c,
		policy: p,
		log:    log,
		cfg:    cfg,
	}
}

func (s *middlewareServiceImpl) GetSession(id string) (*model.SessionRecord, error) {
	decoded, decodeErr := base64.RawURLEncoding.DecodeString(id)
	if decodeErr != nil || len(decoded) != 32 {
		return nil, ErrSessionMissing
	}

	var record model.SessionRecord
	err := s.cache.ReadAndRenewSession(utils.ToSessionCacheKey(id), time.Now().Unix(), 7*24*3600, &record)
	if errors.Is(err, redis.Nil) {
		return nil, ErrSessionMissing
	}
	if err != nil {
		return nil, err
	}

	if record.UserId == "" || record.CSRFToken == "" || record.ExpiresAt <= time.Now().Unix() {
		return nil, ErrSessionMissing
	}

	return &record, nil
}

func (s *middlewareServiceImpl) VerifyExternalToken(token string) (string, error) {
	subject, jti, err := utils.JwtParseExternalToken(
		token,
		s.cfg.GetJwt().AccessTokenSecret,
		s.cfg.GetServer().Name,
	)
	if err != nil {
		return "", ErrExternalMissing
	}

	var record struct {
		SubjectID string `json:"subject_id"`
	}
	if err = s.cache.GetValue(utils.ToExternalTokenCacheKey(jti), &record); errors.Is(err, redis.Nil) {
		return "", ErrExternalMissing
	}
	if err != nil {
		return "", err
	}
	if record.SubjectID != subject {
		return "", ErrExternalMissing
	}

	return subject, nil
}

func (s *middlewareServiceImpl) GetMe(id string) (*model.UserDto, error) {
	user, err := s.repo.GetById(id)
	if err != nil {
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
