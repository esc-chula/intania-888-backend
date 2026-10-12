package policy

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/esc-chula/intania-888-backend/internal/security"
)

// Service evaluates identities and manages access policies.
type Service struct {
	repo  Repository
	cache SnapshotCache
	log   *zap.Logger
}

// NewService constructs a policy service with optional best-effort caching.
func NewService(repo Repository, cache SnapshotCache, log *zap.Logger) *Service {
	if log == nil {
		log = zap.NewNop()
	}

	return &Service{
		repo:  repo,
		cache: cache,
		log:   log,
	}
}

// EvaluateLogin checks blacklist rules before student, admin, and allowlist access.
func (s *Service) EvaluateLogin(ctx context.Context, email, googleSubject, role string) (security.PolicyDecision, error) {
	policies, err := s.loadSnapshot(ctx)
	if err != nil {
		return security.PolicyDecision{}, err
	}
	email = security.NormalizeEmail(email)
	googleSubject = security.NormalizeGoogleSubject(googleSubject)

	if matchesBlacklist(policies, email, googleSubject) {
		return security.PolicyDecision{Blacklisted: true}, nil
	}
	allowed := security.IsStudentEmail(email) || security.IsAdminRole(role) || matchesAllowlist(policies, email)

	return security.PolicyDecision{Allowed: allowed}, nil
}

// IsBlacklisted checks active email and Google subject blacklist entries.
func (s *Service) IsBlacklisted(ctx context.Context, email, userID string) (bool, error) {
	policies, err := s.loadSnapshot(ctx)
	if err != nil {
		return false, err
	}

	return matchesBlacklist(policies, security.NormalizeEmail(email), security.NormalizeGoogleSubject(userID)), nil
}

// List validates filters and returns a page of policies.
func (s *Service) List(ctx context.Context, filter ListFilter) (ListResult, error) {
	if filter.Status == "" {
		filter.Status = StatusActive
	}
	if filter.Status != StatusActive && filter.Status != StatusInactive && filter.Status != StatusAll {
		return ListResult{}, ErrInvalidPolicy
	}
	if filter.Kind != "" && filter.Kind != KindAllowlist && filter.Kind != KindBlacklist {
		return ListResult{}, ErrInvalidPolicy
	}
	if filter.PrincipalType != "" &&
		filter.PrincipalType != PrincipalEmail &&
		filter.PrincipalType != PrincipalGoogleSubject {
		return ListResult{}, ErrInvalidPolicy
	}
	if filter.Limit <= 0 || filter.Limit > 200 {
		filter.Limit = 50
	}
	if filter.Offset < 0 {
		return ListResult{}, ErrInvalidPolicy
	}

	return s.repo.List(ctx, filter)
}

// Create normalizes and creates a policy, preserving a successful write if cache refresh fails.
func (s *Service) Create(ctx context.Context, input CreateInput) (*AccessPolicy, error) {
	normalized, err := ValidateCreateInput(input)
	if err != nil {
		return nil, err
	}
	if existing, findErr := s.repo.FindByIdentity(ctx, normalized.Kind, normalized.PrincipalType, normalized.Principal); findErr == nil &&
		existing != nil {
		return nil, ErrPolicyConflict
	} else if findErr != nil && !errors.Is(findErr, ErrPolicyNotFound) {
		return nil, findErr
	}

	policy := &AccessPolicy{
		ID:            uuid.NewString(),
		Kind:          normalized.Kind,
		PrincipalType: normalized.PrincipalType,
		Principal:     normalized.Principal,
		Reason:        normalized.Reason,
		Enabled:       true,
		ExpiresAt:     normalized.ExpiresAt,
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}
	if err := s.repo.Create(ctx, policy); err != nil {
		return nil, err
	}
	s.refreshCacheBestEffort(ctx, "create")

	return policy, nil
}

// Update changes only mutable policy fields.
func (s *Service) Update(ctx context.Context, id string, input UpdateInput) (*AccessPolicy, error) {
	policy, err := s.repo.FindByID(ctx, id)
	if errors.Is(err, ErrPolicyNotFound) {
		return nil, ErrPolicyNotFound
	}
	if err != nil {
		return nil, err
	}
	if !input.ReasonSet && !input.ExpiresAtSet && !input.EnabledSet {
		return nil, ErrInvalidPolicy
	}
	if input.ReasonSet {
		policy.Reason = strings.TrimSpace(input.Reason)
		if policy.Reason == "" || len(policy.Reason) > 500 {
			return nil, ErrInvalidPolicy
		}
	}
	if input.ExpiresAtSet {
		if input.ExpiresAt != nil && !input.ExpiresAt.After(time.Now()) {
			return nil, ErrInvalidPolicy
		}
		policy.ExpiresAt = input.ExpiresAt
	}
	if input.EnabledSet {
		policy.Enabled = input.Enabled
	}
	policy.UpdatedAt = time.Now().UTC()
	if err := s.repo.Update(ctx, policy); err != nil {
		return nil, err
	}
	s.refreshCacheBestEffort(ctx, "update")

	return policy, nil
}

// Disable idempotently disables an existing policy.
func (s *Service) Disable(ctx context.Context, id string) (*AccessPolicy, error) {
	policy, err := s.repo.FindByID(ctx, id)
	if errors.Is(err, ErrPolicyNotFound) {
		return nil, ErrPolicyNotFound
	}
	if err != nil {
		return nil, err
	}
	if !policy.Enabled {
		return policy, nil
	}
	policy.Enabled = false
	policy.UpdatedAt = time.Now().UTC()
	if err := s.repo.Update(ctx, policy); err != nil {
		return nil, err
	}
	s.refreshCacheBestEffort(ctx, "disable")

	return policy, nil
}

func (s *Service) refreshCacheBestEffort(ctx context.Context, operation string) {
	if err := s.RefreshCache(ctx); err != nil {
		s.log.Warn("Policy mutation committed but cache refresh failed",
			zap.String("operation", operation),
			zap.Error(err),
		)
	}
}

// RefreshCache replaces the active snapshot or invalidates it when refresh fails.
func (s *Service) RefreshCache(ctx context.Context) error {
	if s.cache == nil {
		return nil
	}
	policies, err := s.repo.ListActive(ctx, time.Now())
	if err != nil {
		if cleanupErr := s.cache.Delete(ctx); cleanupErr != nil {
			s.log.Warn("Unable to invalidate access policy cache", zap.Error(cleanupErr))
		}

		return fmt.Errorf("%w: refresh policy snapshot: %w", security.ErrPolicyUnavailable, err)
	}
	if err := s.cache.Store(ctx, policies); err != nil {
		if cleanupErr := s.cache.Delete(ctx); cleanupErr != nil {
			s.log.Warn("Unable to invalidate access policy cache", zap.Error(cleanupErr))
		}

		return fmt.Errorf("%w: write policy snapshot: %w", security.ErrPolicyUnavailable, err)
	}

	return nil
}

func (s *Service) loadSnapshot(ctx context.Context) ([]*AccessPolicy, error) {
	var policies []*AccessPolicy
	if s.cache != nil {
		if cached, err := s.cache.Load(ctx); err == nil {
			return cached, nil
		}
	}
	policies, err := s.repo.ListActive(ctx, time.Now())
	if err != nil {
		return nil, fmt.Errorf("%w: load policy snapshot: %w", security.ErrPolicyUnavailable, err)
	}
	if s.cache != nil {
		if err := s.cache.Store(ctx, policies); err != nil {
			s.log.Warn("Unable to warm access policy cache", zap.Error(err))
		}
	}

	return policies, nil
}

// ValidateCreateInput normalizes and validates a policy for service or bootstrap use.
func ValidateCreateInput(input CreateInput) (CreateInput, error) {
	input.Kind = strings.ToLower(strings.TrimSpace(input.Kind))
	input.PrincipalType = strings.ToLower(strings.TrimSpace(input.PrincipalType))
	input.Reason = strings.TrimSpace(input.Reason)
	if input.Kind != KindAllowlist && input.Kind != KindBlacklist {
		return CreateInput{}, ErrInvalidPolicy
	}
	if input.PrincipalType != PrincipalEmail && input.PrincipalType != PrincipalGoogleSubject {
		return CreateInput{}, ErrInvalidPolicy
	}
	if input.Kind == KindAllowlist && input.PrincipalType != PrincipalEmail {
		return CreateInput{}, ErrInvalidPolicy
	}
	if input.PrincipalType == PrincipalEmail {
		input.Principal = security.NormalizeEmail(input.Principal)
		parsed, err := mail.ParseAddress(input.Principal)
		if err != nil ||
			parsed.Address != input.Principal ||
			len(input.Principal) > 320 ||
			security.IsStudentEmail(input.Principal) &&
				input.Kind == KindAllowlist {
			return CreateInput{}, ErrInvalidPolicy
		}
	} else {
		input.Principal = security.NormalizeGoogleSubject(input.Principal)
		if input.Principal == "" || strings.IndexFunc(input.Principal, func(r rune) bool {
			return r == ' ' || r == '\t' || r == '\r' || r == '\n'
		}) >= 0 || len(input.Principal) > 320 {
			return CreateInput{}, ErrInvalidPolicy
		}
	}
	if input.Reason == "" || len(input.Reason) > 500 {
		return CreateInput{}, ErrInvalidPolicy
	}
	if input.ExpiresAt != nil && !input.ExpiresAt.After(time.Now()) {
		return CreateInput{}, ErrInvalidPolicy
	}

	return input, nil
}

func matchesBlacklist(policies []*AccessPolicy, email, googleSubject string) bool {
	now := time.Now()
	for _, policy := range policies {
		if !activeAt(policy, now) || policy.Kind != KindBlacklist {
			continue
		}
		if policy.PrincipalType == PrincipalEmail && policy.Principal == email {
			return true
		}
		if policy.PrincipalType == PrincipalGoogleSubject && policy.Principal == googleSubject {
			return true
		}
	}

	return false
}

func matchesAllowlist(policies []*AccessPolicy, email string) bool {
	now := time.Now()
	for _, policy := range policies {
		if activeAt(policy, now) &&
			policy.Kind == KindAllowlist &&
			policy.PrincipalType == PrincipalEmail &&
			policy.Principal == email {
			return true
		}
	}

	return false
}

func activeAt(policy *AccessPolicy, now time.Time) bool {
	return policy != nil && policy.Enabled && (policy.ExpiresAt == nil || policy.ExpiresAt.After(now))
}
