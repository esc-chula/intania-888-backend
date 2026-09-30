package middleware

import (
	"context"
	"encoding/base64"
	"errors"
	"slices"
	"time"

	"github.com/esc-chula/intania-888-backend/internal/identity"
	"github.com/esc-chula/intania-888-backend/internal/security"
	"github.com/esc-chula/intania-888-backend/internal/value"
	"github.com/esc-chula/intania-888-backend/pkg/config"
)

// Service verifies request identities using account and session persistence.
type Service struct {
	repo   Repository
	cache  SessionStore
	policy security.AccessPolicyChecker
	cfg    config.Config
	grants DelegationReader
}

// NewService binds the required account, session, configuration and policy dependencies.
func NewService(repo Repository, sessions SessionStore, cfg config.Config, policy security.AccessPolicyChecker) *Service {
	reader, _ := sessions.(DelegationReader)

	return &Service{repo: repo, cache: sessions, cfg: cfg, policy: policy, grants: reader}
}

// GetSession validates the opaque session ID and retrieves its renewed server-side record.
// Malformed IDs or incomplete and expired records return ErrSessionMissing.
func (s *Service) GetSession(ctx context.Context, id string) (*security.Session, error) {
	decoded, decodeErr := base64.RawURLEncoding.DecodeString(id)
	if decodeErr != nil || len(decoded) != security.OpaqueTokenBytes {
		return nil, ErrSessionMissing
	}

	record, err := s.cache.ReadAndRenewSession(ctx, security.ToSessionCacheKey(id), time.Now().Unix(), s.cfg.GetSession().IdleTTLSeconds)
	if err != nil {
		return nil, err
	}

	if record.UserID == "" || record.CSRFToken == "" || record.ExpiresAt <= time.Now().Unix() {
		return nil, ErrSessionMissing
	}

	return record, nil
}

// VerifyExternalToken checks JWT claims and the active subject binding used for revocation.
func (s *Service) VerifyExternalToken(ctx context.Context, token string) (string, error) {
	subject, jti, err := security.JWTParseExternalToken(
		token,
		s.cfg.GetJWT().AccessTokenSecret,
		s.cfg.GetServer().Name,
	)
	if err != nil {
		return "", ErrExternalMissing
	}

	storedSubject, err := s.cache.GetExternalSubject(ctx, security.ToExternalTokenCacheKey(jti))
	if err != nil {
		return "", err
	}
	if storedSubject != subject {
		return "", ErrExternalMissing
	}

	return subject, nil
}

// GetMe loads the account and converts its minor-unit balance to the shared profile value.
func (s *Service) GetMe(ctx context.Context, id string) (*identity.Profile, error) {
	user, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	return &identity.Profile{
		ID:            user.ID,
		Name:          user.Name,
		Email:         user.Email,
		RoleID:        user.RoleID,
		RemainingCoin: value.MustMoneyFromMinor(user.RemainingCoin),
		GroupID:       user.GroupID,
		NickName:      user.NickName,
	}, nil
}

// IsBlacklisted delegates current email and account identity checks to the shared access policy.
func (s *Service) IsBlacklisted(ctx context.Context, email, userID string) (bool, error) {
	return s.policy.IsBlacklisted(ctx, email, userID)
}

// ErrExternalScope indicates that a delegated credential lacks a required permission.
var ErrExternalScope = errors.New("external scope missing")

// VerifyScopedExternalToken checks the active grant, current policy and endpoint permission.
func (s *Service) VerifyScopedExternalToken(ctx context.Context, token, scope string) (string, error) {
	claims, err := security.ParseDelegatedToken(token, s.cfg.GetJWT().AccessTokenSecret, s.cfg.GetServer().Name)
	if err != nil {
		return s.VerifyExternalToken(ctx, token)
	}
	if s.grants == nil {
		return "", ErrExternalMissing
	}
	registry := s.cfg.GetOAuth().Registry
	app, ok := registry.Application(claims.ClientID)
	if !ok || app.Mode != config.CodeApplication {
		return "", ErrExternalMissing
	}
	grant, err := s.grants.ReadDelegation(ctx, claims.DelegationID, registry.Lifetimes.Idle)
	if err != nil {
		return "", err
	}
	if grant.UserID != claims.Subject || grant.ClientID != claims.ClientID {
		return "", ErrExternalMissing
	}
	if scope == "" || !slices.Contains(app.AllowedScopes, scope) || !slices.Contains(grant.Scopes, scope) || !slices.Contains(claims.Scopes, scope) {
		return "", ErrExternalScope
	}
	user, err := s.repo.GetByID(ctx, grant.UserID)
	if errors.Is(err, identity.ErrUserNotFound) {
		return "", ErrExternalMissing
	}
	if err != nil {
		return "", err
	}
	if user == nil {
		return "", ErrExternalMissing
	}
	decision, err := s.policy.EvaluateLogin(ctx, user.Email, user.ID, user.RoleID)
	if err != nil {
		return "", err
	}
	if !decision.Allowed || decision.Blacklisted {
		return "", ErrExternalMissing
	}
	return claims.Subject, nil
}
