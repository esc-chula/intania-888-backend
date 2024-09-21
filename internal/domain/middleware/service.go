package middleware

import (
	"errors"

	"github.com/wiraphatys/intania888/internal/domain/user"
	"github.com/wiraphatys/intania888/internal/model"
	"github.com/wiraphatys/intania888/pkg/cache"
	"github.com/wiraphatys/intania888/pkg/config"
	"github.com/wiraphatys/intania888/utils"
	"go.uber.org/zap"
)

type middlewareServiceImpl struct {
	userRepo user.UserRepository
	cache    *cache.RedisClient
	log      *zap.Logger
	cfg      config.Config
}

func NewMiddlewareService(userRepo user.UserRepository, cache *cache.RedisClient, log *zap.Logger, cfg config.Config) MiddlewareService {
	return &middlewareServiceImpl{
		userRepo: userRepo,
		cache:    cache,
		log:      log,
		cfg:      cfg,
	}
}

func (u *middlewareServiceImpl) VerifyToken(token string) (*string, error) {
	claim, err := utils.JwtParseToken(token, u.cfg.GetJwt().AccessTokenSecret)
	if err != nil {
		u.log.Named("VerifyToken").Error("Parsing token: ", zap.Error(err))
		return nil, errors.New("invalid token")
	}

	// get userId in token
	userId, ok := claim["sub"].(string)
	if !ok {
		u.log.Named("VerifyToken").Error("Getting user_id from claim: ", zap.Error(errors.New("error while getting user_id from claim")))
		return nil, errors.New("user id not found in token")
	}

	var credential model.CredentialDto
	err = u.cache.GetValue(utils.ToAccessCacheKey(userId), &credential)
	if err != nil {
		u.log.Named("ValidateToken").Error("GetValue: ", zap.Error(err))
		return nil, err
	}
	if token != credential.AccessToken {
		return nil, errors.New("invalid token")
	}

	u.log.Named("VerifyToken").Info("Success: ", zap.String("user_id", userId))
	return &userId, nil
}

func (s *middlewareServiceImpl) GetMe(userId string) (*model.UserDto, error) {
	user, err := s.userRepo.GetById(userId)
	if err != nil {
		s.log.Named("GetMe").Error("Get user by id: ", zap.Error(err))
		return nil, err
	}

	return &model.UserDto{
		Id:        user.Id,
		Name:      user.Name,
		Email:     user.Email,
		RoleId:    user.RoleId,
		CreatedAt: user.CreatedAt,
	}, nil
}
