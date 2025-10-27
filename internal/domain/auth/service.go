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

func (s *authServiceImpl) GetOAuthUrl(redirectTo string) (string, error) {
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

	if redirectTo != "" {
		parameters.Add("state", redirectTo)
	}

	URL.RawQuery = parameters.Encode()
	urlString := URL.String()

	s.log.Named("GetGoogleLoginUrl").Info("Success: ", zap.String("url", urlString), zap.String("redirect_to", redirectTo))
	return urlString, nil
}

func (s *authServiceImpl) VerifyOAuthLogin(code string) (*model.CredentialDto, error) {
	userInfo, err := s.oauthClient.GetUserInfo(code)
	if err != nil {
		s.log.Named("VerifyOAuthLogin").Error("Get user info: ", zap.Error(err))
		return nil, err
	}

	allowedEmails := []string{
		"phanthawasjira@gmail.com",
		"bububiib@gmail.com",
		"pear.nataya49@gmail.com",
	}

	isAllowed := false
	if strings.HasSuffix(userInfo.Email, "@student.chula.ac.th") {
		isAllowed = true
	}
	for _, email := range allowedEmails {
		if userInfo.Email == email {
			isAllowed = true
			break
		}
	}

	if !isAllowed {
		s.log.Named("VerifyOAuthLogin").Warn("Please login with Chula student email",
			zap.String("email", userInfo.Email))
		return nil, gorm.ErrInvalidData
	}

	blacklistedEmails := []string{
		"6530162621@student.chula.ac.th",
		"6633129621@student.chula.ac.th",
		"6733023821@student.chula.ac.th",
		"6630054621@student.chula.ac.th",
		"6538004621@student.chula.ac.th",
		"6733291621@student.chula.ac.th",
		"6430039021@student.chula.ac.th",
	}

	for _, email := range blacklistedEmails {
		if userInfo.Email == email {
			s.log.Named("VerifyOAuthLogin").Warn("Idiot",
				zap.String("email", userInfo.Email))
			return nil, gorm.ErrInvalidData
		}
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

func (s *authServiceImpl) IsAllowedRedirect(redirectUrl string) bool {
	parsedUrl, err := url.Parse(redirectUrl)
	if err != nil {
		s.log.Named("IsAllowedRedirect").Warn("Invalid URL format", zap.String("url", redirectUrl), zap.Error(err))
		return false
	}

	if parsedUrl.Scheme != "http" && parsedUrl.Scheme != "https" {
		s.log.Named("IsAllowedRedirect").Warn("Invalid scheme", zap.String("url", redirectUrl))
		return false
	}

	hostname := parsedUrl.Hostname()

	allowedDomains := []string{
		"localhost",
		"127.0.0.1",
		"888.intania.org",
	}

	isAllowed := false
	for _, allowed := range allowedDomains {
		if hostname == allowed {
			isAllowed = true
			break
		}
	}

	if !isAllowed {
		s.log.Named("IsAllowedRedirect").Warn("Domain not in whitelist", zap.String("domain", hostname))
		return false
	}

	if hostname == "888.intania.org" && parsedUrl.Scheme != "https" {
		s.log.Named("IsAllowedRedirect").Error("Production domain must use HTTPS", zap.String("url", redirectUrl))
		return false
	}

	s.log.Named("IsAllowedRedirect").Info("Redirect allowed", zap.String("url", redirectUrl))
	return true
}

func (s *authServiceImpl) GetFrontendUrl() string {
	return s.cfg.GetOAuth().FrontendUrl
}
