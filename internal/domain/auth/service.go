package auth

import (
	"crypto/subtle"
	"errors"
	"net/url"
	"time"

	"github.com/esc-chula/intania-888-backend/internal/domain/user"
	"github.com/esc-chula/intania-888-backend/internal/model"
	"github.com/esc-chula/intania-888-backend/internal/security"
	"github.com/esc-chula/intania-888-backend/pkg/config"
	"github.com/esc-chula/intania-888-backend/pkg/oauth"
	"github.com/esc-chula/intania-888-backend/utils"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
	"gorm.io/gorm"
)

const (
	sessionIdleSeconds     = 7 * 24 * 3600
	sessionAbsoluteSeconds = 30 * 24 * 3600
	externalTokenSeconds   = 3600
)

type authServiceImpl struct {
	authRepo    AuthRepository
	userRepo    user.UserRepository
	policy      security.AccessPolicyChecker
	cfg         config.Config
	log         *zap.Logger
	oauthClient oauth.GoogleOAuthClient
}

func NewAuthService(
	repo AuthRepository,
	users user.UserRepository,
	cfg config.Config,
	log *zap.Logger,
	client oauth.GoogleOAuthClient,
	policy security.AccessPolicyChecker,
) AuthService {
	if policy == nil {
		policy = security.DefaultPolicyChecker{}
	}
	return &authServiceImpl{
		authRepo:    repo,
		userRepo:    users,
		policy:      policy,
		cfg:         cfg,
		log:         log,
		oauthClient: client,
	}
}

func (s *authServiceImpl) StartOAuthLogin() (*OAuthLogin, error) {
	if s.oauthClient == nil {
		return nil, errors.New("OAuth client is not configured")
	}
	cfg := s.oauthClient.OAuthConfig()
	if cfg == nil || cfg.ClientID == "" || cfg.RedirectURL == "" || cfg.Endpoint.AuthURL == "" {
		return nil, errors.New("OAuth client is not configured")
	}
	parsed, err := url.Parse(cfg.Endpoint.AuthURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil {
		return nil, errors.New("invalid OAuth authorization URL")
	}
	state, err := utils.NewOpaqueToken(32)
	if err != nil {
		return nil, err
	}
	verifier, err := utils.NewOpaqueToken(32)
	if err != nil {
		return nil, err
	}
	ttl := s.cfg.GetOAuth().StateExpiration
	if ttl <= 0 {
		ttl = 600
	}
	if err = s.authRepo.SetCacheValue(
		utils.ToOAuthStateCacheKey(state),
		model.OAuthStateRecord{CodeVerifier: verifier},
		ttl,
	); err != nil {
		return nil, err
	}
	return &OAuthLogin{
		URL: cfg.AuthCodeURL(
			state,
			oauth2.S256ChallengeOption(verifier),
			oauth2.SetAuthURLParam("hd", "student.chula.ac.th"),
		),
		State: state,
	}, nil
}

func (s *authServiceImpl) VerifyOAuthLogin(
	code, state, cookieState, previousSessionID string,
) (*SessionCredentials, error) {
	if code == "" || state == "" || cookieState == "" ||
		subtle.ConstantTimeCompare([]byte(state), []byte(cookieState)) != 1 {
		return nil, ErrInvalidOAuthState
	}
	var record model.OAuthStateRecord
	if err := s.authRepo.ConsumeCacheValue(utils.ToOAuthStateCacheKey(state), &record); err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, ErrInvalidOAuthState
		}
		return nil, err
	}
	if record.CodeVerifier == "" {
		return nil, ErrInvalidOAuthState
	}
	info, err := s.oauthClient.GetUserInfo(code, record.CodeVerifier)
	if err != nil {
		return nil, err
	}
	if info == nil || !info.VerifiedEmail {
		return nil, ErrUnverifiedEmail
	}
	if info.Id == "" {
		return nil, errors.New("google user ID is empty")
	}
	email := security.NormalizeEmail(info.Email)
	existing, err := s.userRepo.GetByEmail(email)
	isNew := false
	switch {
	case err == nil:
	case errors.Is(err, gorm.ErrRecordNotFound):
		existing = nil
	default:
		return nil, err
	}
	role := ""
	if existing != nil {
		role = existing.RoleId
	}
	decision, err := s.policy.EvaluateLogin(email, info.Id, role)
	if err != nil {
		return nil, err
	}
	if decision.Blacklisted || !decision.Allowed {
		return nil, ErrEmailNotAllowed
	}
	if existing == nil {
		isNew = true
		existing = &model.User{
			Id:            info.Id,
			Email:         email,
			Name:          info.Name,
			RoleId:        security.RoleUser,
			RemainingCoin: 888_88,
		}
		if err := s.userRepo.Create(existing); err != nil {
			return nil, err
		}
	}
	id, err := utils.NewOpaqueToken(32)
	if err != nil {
		return nil, err
	}
	csrf, err := utils.NewOpaqueToken(32)
	if err != nil {
		return nil, err
	}
	now := time.Now().Unix()
	session := model.SessionRecord{
		UserId:    existing.Id,
		CreatedAt: now,
		ExpiresAt: now + sessionAbsoluteSeconds,
		CSRFToken: csrf,
	}
	if err := s.authRepo.RotateSession(
		utils.ToUserSessionCacheKey(existing.Id),
		utils.ToSessionCacheKey(id),
		utils.ToSessionCacheKey(previousSessionID),
		session,
		sessionIdleSeconds,
	); err != nil {
		return nil, err
	}
	return &SessionCredentials{SessionID: id, IsNewUser: isNew}, nil
}

func (s *authServiceImpl) Logout(sessionID string) error {
	if sessionID == "" {
		return nil
	}
	return s.authRepo.DeleteSession(utils.ToSessionCacheKey(sessionID))
}

func (s *authServiceImpl) GetPostLoginRedirectURL() string {
	return s.cfg.GetOAuth().PostLoginRedirectUrl
}

func (s *authServiceImpl) IssueExternalToken(subjectID string) (string, string, error) {
	subject, err := s.userRepo.GetById(subjectID)
	if err != nil {
		return "", "", err
	}
	if subject == nil {
		return "", "", gorm.ErrRecordNotFound
	}
	jti, err := utils.NewOpaqueToken(32)
	if err != nil {
		return "", "", err
	}
	token, err := utils.JwtSignExternalToken(
		subjectID,
		jti,
		s.cfg.GetJwt().AccessTokenSecret,
		s.cfg.GetServer().Name,
		externalTokenSeconds,
	)
	if err != nil {
		return "", "", err
	}
	key := utils.ToExternalTokenCacheKey(jti)
	if err := s.authRepo.SetCacheValue(key, ExternalTokenRecord{SubjectID: subjectID}, externalTokenSeconds); err != nil {
		return "", "", err
	}
	return token, jti, nil
}

func (s *authServiceImpl) RevokeExternalToken(jti string) error {
	if jti == "" {
		return errors.New("external token ID is required")
	}
	return s.authRepo.DeleteSession(utils.ToExternalTokenCacheKey(jti))
}
