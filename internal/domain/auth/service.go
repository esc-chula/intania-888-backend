package auth

import (
	"crypto/subtle"
	"errors"
	"net/url"

	"github.com/esc-chula/intania-888-backend/internal/domain/user"
	"github.com/esc-chula/intania-888-backend/internal/model"
	"github.com/esc-chula/intania-888-backend/internal/security"
	"github.com/esc-chula/intania-888-backend/pkg/config"
	"github.com/esc-chula/intania-888-backend/pkg/oauth"
	"github.com/esc-chula/intania-888-backend/utils"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
	"gorm.io/gorm"
)

const (
	defaultOAuthStateExpiration = 10 * 60
	refreshTokenActive          = "active"
	refreshTokenUsed            = "used"
)

type authServiceImpl struct {
	authRepo    AuthRepository
	userRepo    user.UserRepository
	policy      security.AccessPolicyChecker
	cfg         config.Config
	log         *zap.Logger
	oauthClient oauth.GoogleOAuthClient
}

func NewAuthService(authRepo AuthRepository, userRepo user.UserRepository, cfg config.Config, log *zap.Logger, oauthClient oauth.GoogleOAuthClient, policies ...security.AccessPolicyChecker) AuthService {
	policyChecker := security.AccessPolicyChecker(security.DefaultPolicyChecker{})
	if len(policies) > 0 && policies[0] != nil {
		policyChecker = policies[0]
	}
	return &authServiceImpl{
		authRepo:    authRepo,
		userRepo:    userRepo,
		policy:      policyChecker,
		cfg:         cfg,
		log:         log,
		oauthClient: oauthClient,
	}
}

func (s *authServiceImpl) StartOAuthLogin() (*OAuthLogin, error) {
	if s.oauthClient == nil {
		return nil, errors.New("OAuth client is not configured")
	}
	oauthConfig := s.oauthClient.OAuthConfig()
	if oauthConfig == nil || oauthConfig.ClientID == "" || oauthConfig.RedirectURL == "" || oauthConfig.Endpoint.AuthURL == "" {
		return nil, errors.New("OAuth client is not configured")
	}
	authorizationURL, err := url.Parse(oauthConfig.Endpoint.AuthURL)
	if err != nil || authorizationURL.Scheme == "" || authorizationURL.Host == "" || authorizationURL.User != nil {
		return nil, errors.New("invalid OAuth authorization URL")
	}

	state, err := utils.NewOpaqueToken(32)
	if err != nil {
		return nil, err
	}
	codeVerifier, err := utils.NewOpaqueToken(32)
	if err != nil {
		return nil, err
	}

	if err := s.authRepo.SetCacheValue(
		utils.ToOAuthStateCacheKey(state),
		model.OAuthStateRecord{CodeVerifier: codeVerifier},
		s.oauthStateExpiration(),
	); err != nil {
		return nil, err
	}

	authURL := oauthConfig.AuthCodeURL(
		state,
		oauth2.S256ChallengeOption(codeVerifier),
		oauth2.SetAuthURLParam("hd", "student.chula.ac.th"),
	)
	return &OAuthLogin{URL: authURL, State: state}, nil
}

func (s *authServiceImpl) VerifyOAuthLogin(code, state, cookieState string) (*SessionCredentials, error) {
	if code == "" || state == "" || cookieState == "" || subtle.ConstantTimeCompare([]byte(state), []byte(cookieState)) != 1 {
		return nil, ErrInvalidOAuthState
	}

	var stateRecord model.OAuthStateRecord
	if err := s.authRepo.ConsumeCacheValue(utils.ToOAuthStateCacheKey(state), &stateRecord); err != nil || stateRecord.CodeVerifier == "" {
		return nil, ErrInvalidOAuthState
	}

	userInfo, err := s.oauthClient.GetUserInfo(code, stateRecord.CodeVerifier)
	if err != nil {
		s.log.Named("VerifyOAuthLogin").Error("Get Google user info", zap.Error(err))
		return nil, err
	}
	if userInfo == nil || !userInfo.VerifiedEmail {
		return nil, ErrUnverifiedEmail
	}

	if userInfo.Id == "" {
		return nil, errors.New("google user ID is empty")
	}

	email := security.NormalizeEmail(userInfo.Email)
	existedUser, err := s.userRepo.GetByEmail(email)
	isNewUser := false
	switch {
	case err == nil:
	case errors.Is(err, gorm.ErrRecordNotFound):
		existedUser = nil
	case err != nil:
		s.log.Named("VerifyOAuthLogin").Error("Find user", zap.Error(err))
		return nil, err
	}

	role := ""
	if existedUser != nil {
		role = existedUser.RoleId
	}
	decision, err := s.policy.EvaluateLogin(email, userInfo.Id, role)
	if err != nil {
		return nil, err
	}
	if decision.Blacklisted || !decision.Allowed {
		return nil, ErrEmailNotAllowed
	}

	if existedUser == nil {
		isNewUser = true
		existedUser = &model.User{
			Id:            userInfo.Id,
			Email:         email,
			Name:          userInfo.Name,
			RoleId:        security.RoleUser,
			RemainingCoin: 888_88,
		}
		if err := s.userRepo.Create(existedUser); err != nil {
			s.log.Named("VerifyOAuthLogin").Error("Create user", zap.Error(err))
			return nil, err
		}
	}

	return s.createSession(existedUser.Id, existedUser.RoleId, isNewUser)
}

func (s *authServiceImpl) createSession(userID, role string, isNewUser bool) (*SessionCredentials, error) {
	sessionID, err := utils.NewOpaqueToken(32)
	if err != nil {
		return nil, err
	}
	refreshToken, err := utils.JwtSignRefreshToken(s.cfg.GetJwt().RefreshTokenExpiration)
	if err != nil {
		return nil, err
	}
	accessToken, err := utils.JwtSignAccessTokenWithSession(
		userID,
		role,
		sessionID,
		s.cfg.GetJwt().AccessTokenSecret,
		s.cfg.GetServer().Name,
		s.cfg.GetServer().Name,
		s.cfg.GetJwt().AccessTokenExpiration,
	)
	if err != nil {
		return nil, err
	}

	refreshHash := utils.HashOpaqueToken(*refreshToken)
	session := model.SessionRecord{
		Id:               sessionID,
		UserId:           userID,
		Role:             role,
		RefreshTokenHash: refreshHash,
	}
	refreshRecord := model.RefreshTokenRecord{
		SessionId: sessionID,
		UserId:    userID,
		Role:      role,
		Status:    refreshTokenActive,
	}
	refreshTTL := s.cfg.GetJwt().RefreshTokenExpiration
	if refreshTTL <= 0 {
		return nil, errors.New("refresh token expiration must be positive")
	}

	if err := s.authRepo.SetCacheValue(utils.ToSessionCacheKey(sessionID), session, refreshTTL); err != nil {
		return nil, err
	}
	if err := s.authRepo.SetCacheValue(utils.ToRefreshHashCacheKey(refreshHash), refreshRecord, refreshTTL); err != nil {
		_ = s.authRepo.DeleteCacheValue(utils.ToSessionCacheKey(sessionID))
		return nil, err
	}

	return &SessionCredentials{
		AccessToken:  accessToken,
		RefreshToken: *refreshToken,
		ExpiresIn:    int32(s.cfg.GetJwt().AccessTokenExpiration),
		IsNewUser:    isNewUser,
	}, nil
}

func (s *authServiceImpl) RefreshToken(refreshToken string) (*SessionCredentials, error) {
	if refreshToken == "" {
		return nil, ErrInvalidRefresh
	}

	refreshHash := utils.HashOpaqueToken(refreshToken)
	var refreshRecord model.RefreshTokenRecord
	if err := s.authRepo.GetCacheValue(utils.ToRefreshHashCacheKey(refreshHash), &refreshRecord); err != nil {
		return nil, ErrInvalidRefresh
	}
	if refreshRecord.Status == refreshTokenUsed {
		s.revokeSession(refreshRecord.SessionId)
		return nil, ErrRefreshReplay
	}
	if refreshRecord.Status != refreshTokenActive || refreshRecord.SessionId == "" {
		return nil, ErrInvalidRefresh
	}

	var session model.SessionRecord
	if err := s.authRepo.GetCacheValue(utils.ToSessionCacheKey(refreshRecord.SessionId), &session); err != nil {
		return nil, ErrInvalidRefresh
	}
	if session.Id != refreshRecord.SessionId || session.UserId != refreshRecord.UserId || session.RefreshTokenHash != refreshHash {
		s.revokeSession(refreshRecord.SessionId)
		return nil, ErrRefreshReplay
	}

	currentUser, err := s.userRepo.GetById(session.UserId)
	if err != nil {
		return nil, ErrInvalidRefresh
	}
	blacklisted, err := s.policy.IsBlacklisted(currentUser.Email, currentUser.Id)
	if err != nil {
		return nil, err
	}
	if blacklisted {
		s.revokeSession(session.Id)
		return nil, ErrEmailNotAllowed
	}
	storedSession := session
	updatedSession := session
	updatedSession.Role = currentUser.RoleId

	newRefreshToken, err := utils.JwtSignRefreshToken(s.cfg.GetJwt().RefreshTokenExpiration)
	if err != nil {
		return nil, err
	}
	newRefreshHash := utils.HashOpaqueToken(*newRefreshToken)
	accessToken, err := utils.JwtSignAccessTokenWithSession(
		updatedSession.UserId,
		updatedSession.Role,
		updatedSession.Id,
		s.cfg.GetJwt().AccessTokenSecret,
		s.cfg.GetServer().Name,
		s.cfg.GetServer().Name,
		s.cfg.GetJwt().AccessTokenExpiration,
	)
	if err != nil {
		return nil, err
	}

	refreshTTL := s.cfg.GetJwt().RefreshTokenExpiration
	usedRefreshRecord := refreshRecord
	usedRefreshRecord.Status = refreshTokenUsed
	newSession := updatedSession
	newSession.RefreshTokenHash = newRefreshHash
	newRefreshRecord := model.RefreshTokenRecord{
		SessionId: updatedSession.Id,
		UserId:    updatedSession.UserId,
		Role:      updatedSession.Role,
		Status:    refreshTokenActive,
	}

	rotated, err := s.authRepo.CompareAndSwapCacheValues(
		map[string]interface{}{
			utils.ToRefreshHashCacheKey(refreshHash):  refreshRecord,
			utils.ToSessionCacheKey(storedSession.Id): storedSession,
		},
		map[string]interface{}{
			utils.ToRefreshHashCacheKey(refreshHash):    usedRefreshRecord,
			utils.ToRefreshHashCacheKey(newRefreshHash): newRefreshRecord,
			utils.ToSessionCacheKey(session.Id):         newSession,
		},
		refreshTTL,
	)
	if err != nil {
		return nil, err
	}
	if !rotated {
		return nil, s.refreshRotationConflict(refreshRecord.SessionId, refreshHash)
	}

	return &SessionCredentials{
		AccessToken:  accessToken,
		RefreshToken: *newRefreshToken,
		ExpiresIn:    int32(s.cfg.GetJwt().AccessTokenExpiration),
	}, nil
}

func (s *authServiceImpl) refreshRotationConflict(sessionID, presentedHash string) error {
	var currentRefreshRecord model.RefreshTokenRecord
	if err := s.authRepo.GetCacheValue(utils.ToRefreshHashCacheKey(presentedHash), &currentRefreshRecord); err != nil {
		return ErrInvalidRefresh
	}
	if currentRefreshRecord.Status == refreshTokenUsed {
		s.revokeSession(currentRefreshRecord.SessionId)
		return ErrRefreshReplay
	}

	var currentSession model.SessionRecord
	if err := s.authRepo.GetCacheValue(utils.ToSessionCacheKey(sessionID), &currentSession); err != nil {
		return ErrInvalidRefresh
	}
	if currentSession.RefreshTokenHash != presentedHash {
		s.revokeSession(sessionID)
		return ErrRefreshReplay
	}
	return ErrInvalidRefresh
}

func (s *authServiceImpl) Logout(sessionID string) error {
	if sessionID == "" {
		return nil
	}

	var session model.SessionRecord
	if err := s.authRepo.GetCacheValue(utils.ToSessionCacheKey(sessionID), &session); err != nil {
		return err
	}
	return s.authRepo.DeleteCacheValues(
		utils.ToRefreshHashCacheKey(session.RefreshTokenHash),
		utils.ToSessionCacheKey(sessionID),
	)
}

func (s *authServiceImpl) revokeSession(sessionID string) {
	if sessionID == "" {
		return
	}
	var session model.SessionRecord
	keys := []string{utils.ToSessionCacheKey(sessionID)}
	if err := s.authRepo.GetCacheValue(utils.ToSessionCacheKey(sessionID), &session); err == nil {
		keys = append(keys, utils.ToRefreshHashCacheKey(session.RefreshTokenHash))
	}
	_ = s.authRepo.DeleteCacheValues(keys...)
}

func (s *authServiceImpl) oauthStateExpiration() int {
	if expiration := s.cfg.GetOAuth().StateExpiration; expiration > 0 {
		return expiration
	}
	return defaultOAuthStateExpiration
}

func (s *authServiceImpl) GetPostLoginRedirectURL() string {
	return s.cfg.GetOAuth().PostLoginRedirectUrl
}
