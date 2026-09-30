package auth

import (
	"context"
	"crypto/subtle"
	"errors"
	"net/url"
	"time"

	"github.com/esc-chula/intania-888-backend/internal/identity"

	"github.com/esc-chula/intania-888-backend/internal/security"
	"github.com/esc-chula/intania-888-backend/pkg/config"
	"github.com/esc-chula/intania-888-backend/pkg/oauth"

	"golang.org/x/oauth2"
)

// Service coordinates OAuth state, account admission, browser sessions, and external credentials.
type Service struct {
	authRepo    Repository
	userRepo    Users
	policy      security.AccessPolicyChecker
	cfg         config.Config
	oauthClient oauth.GoogleOAuthClient
}

// NewService binds authentication persistence, accounts, configuration, Google OAuth, and access policy.
// A nil policy uses the default checker; production wiring supplies the persisted policy service.
func NewService(
	repo Repository,
	users Users,
	cfg config.Config,
	client oauth.GoogleOAuthClient,
	policy security.AccessPolicyChecker,
) *Service {
	if policy == nil {
		policy = security.DefaultPolicyChecker{}
	}

	return &Service{
		authRepo:    repo,
		userRepo:    users,
		policy:      policy,
		cfg:         cfg,
		oauthClient: client,
	}
}

// StartOAuthLogin creates browser-bound state and a PKCE verifier before returning the Google authorization URL.
func (s *Service) StartOAuthLogin(ctx context.Context) (*OAuthLogin, error) {
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

	state, err := security.NewOpaqueToken(security.OpaqueTokenBytes)
	if err != nil {
		return nil, err
	}

	verifier, err := security.NewOpaqueToken(security.OpaqueTokenBytes)
	if err != nil {
		return nil, err
	}

	registry := s.cfg.GetOAuth().Registry
	if registry == nil || registry.Lifetimes.Login <= 0 {
		return nil, errors.New("login transaction lifetime is not configured")
	}

	ttl := registry.Lifetimes.Login

	if err = s.authRepo.StoreOAuthState(
		ctx,
		security.ToOAuthStateCacheKey(state),
		OAuthState{CodeVerifier: verifier},
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

// VerifyOAuthLogin checks the browser state and consumes its PKCE verifier before exchanging the code.
// It applies access policy, creates an admitted account when needed, and replaces the prior browser session.
func (s *Service) VerifyOAuthLogin(
	ctx context.Context,
	code, state, cookieState, previousSessionID string,
) (*SessionCredentials, error) {
	if code == "" || state == "" || cookieState == "" ||
		subtle.ConstantTimeCompare([]byte(state), []byte(cookieState)) != 1 {
		return nil, ErrInvalidOAuthState
	}

	record, err := s.authRepo.ConsumeOAuthState(ctx, security.ToOAuthStateCacheKey(state))
	if err != nil {
		return nil, err
	}
	if record.CodeVerifier == "" {
		return nil, ErrInvalidOAuthState
	}

	info, err := s.oauthClient.GetUserInfo(ctx, code, record.CodeVerifier)
	if err != nil {
		return nil, err
	}
	if info == nil || !info.VerifiedEmail {
		return nil, ErrUnverifiedEmail
	}
	if info.ID == "" {
		return nil, errors.New("google user ID is empty")
	}

	email := security.NormalizeEmail(info.Email)
	existing, err := s.userRepo.GetByEmail(ctx, email)
	isNew := false

	switch {
	case err == nil:
	case errors.Is(err, identity.ErrUserNotFound):
		existing = nil
	default:
		return nil, err
	}

	role := ""
	if existing != nil {
		role = existing.RoleID
	}

	decision, err := s.policy.EvaluateLogin(ctx, email, info.ID, role)
	if err != nil {
		return nil, err
	}
	if decision.Blacklisted || !decision.Allowed {
		return nil, ErrEmailNotAllowed
	}

	if existing == nil {
		isNew = true
		existing = &identity.User{
			ID:            info.ID,
			Email:         email,
			Name:          info.Name,
			RoleID:        security.RoleUser,
			RemainingCoin: 888_88,
		}

		if err := s.userRepo.Create(ctx, existing); err != nil {
			return nil, err
		}
	}

	id, err := security.NewOpaqueToken(security.OpaqueTokenBytes)
	if err != nil {
		return nil, err
	}

	csrf, err := security.NewOpaqueToken(security.OpaqueTokenBytes)
	if err != nil {
		return nil, err
	}

	now := time.Now().Unix()
	sessionConfig := s.cfg.GetSession()
	session := security.Session{
		UserID:    existing.ID,
		CreatedAt: now,
		ExpiresAt: now + int64(sessionConfig.AbsoluteTTLSeconds),
		CSRFToken: csrf,
	}

	if err := s.authRepo.RotateSession(
		ctx,
		security.ToUserSessionCacheKey(existing.ID),
		security.ToSessionCacheKey(id),
		security.ToSessionCacheKey(previousSessionID),
		session,
		sessionConfig.IdleTTLSeconds,
		sessionConfig.AbsoluteTTLSeconds,
	); err != nil {
		return nil, err
	}

	return &SessionCredentials{SessionID: id, UserID: existing.ID, IsNewUser: isNew}, nil
}

// Logout revokes a browser session. An empty ID or already absent session succeeds.
func (s *Service) Logout(ctx context.Context, sessionID string) error {
	if sessionID == "" {
		return nil
	}
	return s.authRepo.DeleteSession(ctx, security.ToSessionCacheKey(sessionID))
}
