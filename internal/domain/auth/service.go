package auth

import (
	"net/url"
	"strings"

	"github.com/esc-chula/intania-888-backend/internal/domain/user"
	"github.com/esc-chula/intania-888-backend/internal/model"
	"github.com/esc-chula/intania-888-backend/pkg/config"
	"github.com/esc-chula/intania-888-backend/pkg/oauth"
	"github.com/esc-chula/intania-888-backend/utils"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type authServiceImpl struct {
	authRepo    AuthRepository
	userRepo    user.UserRepository
	cfg         config.Config
	log         *zap.Logger
	oauthClient oauth.GoogleOAuthClient
}

func NewAuthService(authRepo AuthRepository, userRepo user.UserRepository, cfg config.Config, log *zap.Logger, oauthClient oauth.GoogleOAuthClient) AuthService {
	return &authServiceImpl{
		authRepo:    authRepo,
		userRepo:    userRepo,
		cfg:         cfg,
		log:         log,
		oauthClient: oauthClient,
	}
}

func (s *authServiceImpl) GetOAuthUrl() (string, error) {
	URL, err := url.Parse(s.oauthClient.OAuthConfig().Endpoint.AuthURL)
	if err != nil {
		s.log.Named("GetGoogleLoginUrl").Error("Parse: ", zap.Error(err))
		return "", err
	}
	parameters := url.Values{}
	parameters.Add("client_id", s.oauthClient.OAuthConfig().ClientID)
	parameters.Add("scope", strings.Join(s.oauthClient.OAuthConfig().Scopes, " "))
	parameters.Add("redirect_uri", s.oauthClient.OAuthConfig().RedirectURL)
	parameters.Add("response_type", "code")
	parameters.Add("hd", "student.chula.ac.th")
	URL.RawQuery = parameters.Encode()
	url := URL.String()

	s.log.Named("GetGoogleLoginUrl").Info("Success: ", zap.String("url", url))
	return url, nil
}

func (s *authServiceImpl) VerifyOAuthLogin(code string) (*model.CredentialDto, error) {
	userInfo, err := s.oauthClient.GetUserInfo(code)
	if err != nil {
		s.log.Named("VerifyOAuthLogin").Error("Get user info: ", zap.Error(err))
		return nil, err
	}

	existedUser, err := s.userRepo.GetByEmail(userInfo.Email)
	if err != nil && err == gorm.ErrRecordNotFound {
		s.log.Named("VerifyOAuthLogin").Info("User not found, creating new user")

		role := "USER"
		userToCreate := model.User{
			Id:            userInfo.Id,
			Email:         userInfo.Email,
			Name:          userInfo.Name,
			RoleId:        role,
			RemainingCoin: 888.88,
		}

		if err := s.userRepo.Create(&userToCreate); err != nil {
			s.log.Named("VerifyOAuthLogin").Error("Create user: ", zap.Error(err))
			return nil, err
		}

		accessToken, err := utils.JwtSignAccessToken(userInfo.Id, role, s.cfg.GetJwt().AccessTokenSecret, s.cfg.GetJwt().AccessTokenExpiration)
		if err != nil {
			s.log.Named("VerifyOAuthLogin").Error("Jwt sign access token: ", zap.Error(err))
			return nil, err
		}

		refreshToken, err := utils.JwtSignRefreshToken(s.cfg.GetJwt().RefreshTokenExpiration)
		if err != nil {
			s.log.Named("VerifyOAuthLogin").Error("Jwt sign refresh token: ", zap.Error(err))
			return nil, err
		}

		credential := utils.NewCredentials(*accessToken, *refreshToken, int32(s.cfg.GetJwt().AccessTokenExpiration), true)

		if err := s.authRepo.SetCacheValue(utils.ToAccessCacheKey(userToCreate.Id), credential, s.cfg.GetJwt().AccessTokenExpiration); err != nil {
			s.log.Named("VerifyOAuthLogin").Error("Set access cache value: ", zap.Error(err))
			return nil, err
		}

		if err := s.authRepo.SetCacheValue(utils.ToRefreshCacheKey(*refreshToken), model.RefreshCacheDto{UserId: userToCreate.Id, Role: role}, s.cfg.GetJwt().RefreshTokenExpiration); err != nil {
			s.log.Named("VerifyOAuthLogin").Error("Set access cache value: ", zap.Error(err))
			return nil, err
		}

		return credential, nil
	}

	accessToken, err := utils.JwtSignAccessToken(existedUser.Id, existedUser.RoleId, s.cfg.GetJwt().AccessTokenSecret, s.cfg.GetJwt().AccessTokenExpiration)
	if err != nil {
		s.log.Named("VerifyOAuthLogin").Error("Jwt sign access token: ", zap.Error(err))
		return nil, err
	}

	refreshToken, err := utils.JwtSignRefreshToken(s.cfg.GetJwt().RefreshTokenExpiration)
	if err != nil {
		s.log.Named("VerifyOAuthLogin").Error("Jwt sign refresh token: ", zap.Error(err))
		return nil, err
	}

	credential := utils.NewCredentials(*accessToken, *refreshToken, int32(s.cfg.GetJwt().AccessTokenExpiration), false)

	if err := s.authRepo.SetCacheValue(utils.ToAccessCacheKey(existedUser.Id), credential, s.cfg.GetJwt().AccessTokenExpiration); err != nil {
		s.log.Named("VerifyOAuthLogin").Error("Set access cache value: ", zap.Error(err))
		return nil, err
	}

	if err := s.authRepo.SetCacheValue(utils.ToRefreshCacheKey(*refreshToken), model.RefreshCacheDto{UserId: existedUser.Id, Role: existedUser.RoleId}, s.cfg.GetJwt().RefreshTokenExpiration); err != nil {
		s.log.Named("VerifyOAuthLogin").Error("Set access cache value: ", zap.Error(err))
		return nil, err
	}

	return credential, nil
}

func (s *authServiceImpl) RefreshToken(refreshToken string) (*model.CredentialDto, error) {
	// find in cache
	var refreshCacheDto model.RefreshCacheDto
	var emptyCache model.RefreshCacheDto
	if err := s.authRepo.GetCacheValue(utils.ToRefreshCacheKey(refreshToken), &refreshCacheDto); err != nil {
		s.log.Named("RefreshToken").Error("Get cache value: ", zap.Error(err))
		return nil, err
	} else if refreshCacheDto == emptyCache {
		s.log.Named("RefreshToken").Info("Get cache value: refresh token not found")
		return nil, err
	}

	accessToken, err := utils.JwtSignAccessToken(refreshCacheDto.UserId, refreshCacheDto.Role, s.cfg.GetJwt().AccessTokenSecret, s.cfg.GetJwt().AccessTokenExpiration)
	if err != nil {
		s.log.Named("RefreshToken").Error("Jwt sign access token: ", zap.Error(err))
		return nil, err
	}

	newCredential := utils.NewCredentials(*accessToken, refreshToken, int32(s.cfg.GetJwt().AccessTokenExpiration), false)
	if err := s.authRepo.SetCacheValue(utils.ToAccessCacheKey(refreshCacheDto.UserId), newCredential, s.cfg.GetJwt().AccessTokenExpiration); err != nil {
		s.log.Named("RefreshToken").Error("Set access cache value: ", zap.Error(err))
		return nil, err
	}

	return newCredential, nil
}
