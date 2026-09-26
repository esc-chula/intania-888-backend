package policy

import (
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/esc-chula/intania-888-backend/internal/security"
	"github.com/esc-chula/intania-888-backend/utils"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

const policyCacheTTL = 60

var (
	ErrInvalidPolicy  = errors.New("invalid access policy")
	ErrPolicyNotFound = errors.New("access policy not found")
	ErrPolicyConflict = errors.New("access policy already exists")
)

type service struct {
	repo  Repository
	cache Cache
	log   *zap.Logger
}

func NewService(repo Repository, cache Cache, log *zap.Logger) Service {
	if log == nil {
		log = zap.NewNop()
	}
	return &service{repo: repo, cache: cache, log: log}
}

func (s *service) EvaluateLogin(email, googleSubject, role string) (security.PolicyDecision, error) {
	policies, err := s.loadSnapshot()
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

func (s *service) IsBlacklisted(email, userID string) (bool, error) {
	policies, err := s.loadSnapshot()
	if err != nil {
		return false, err
	}
	return matchesBlacklist(policies, security.NormalizeEmail(email), security.NormalizeGoogleSubject(userID)), nil
}

func (s *service) List(filter ListFilter) (ListResult, error) {
	if filter.Status == "" {
		filter.Status = StatusActive
	}
	if filter.Status != StatusActive && filter.Status != StatusInactive && filter.Status != StatusAll {
		return ListResult{}, ErrInvalidPolicy
	}
	if filter.Kind != "" && filter.Kind != KindAllowlist && filter.Kind != KindBlacklist {
		return ListResult{}, ErrInvalidPolicy
	}
	if filter.PrincipalType != "" && filter.PrincipalType != PrincipalEmail && filter.PrincipalType != PrincipalGoogleSubject {
		return ListResult{}, ErrInvalidPolicy
	}
	if filter.Limit <= 0 || filter.Limit > 200 {
		filter.Limit = 50
	}
	if filter.Offset < 0 {
		return ListResult{}, ErrInvalidPolicy
	}
	return s.repo.List(filter)
}

func (s *service) Create(input CreateInput) (*AccessPolicy, error) {
	normalized, err := ValidateCreateInput(input)
	if err != nil {
		return nil, err
	}
	if existing, findErr := s.repo.FindByIdentity(normalized.Kind, normalized.PrincipalType, normalized.Principal); findErr == nil && existing != nil {
		return nil, ErrPolicyConflict
	} else if findErr != nil && !errors.Is(findErr, gorm.ErrRecordNotFound) {
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
	if err := s.repo.Create(policy); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrPolicyConflict
		}
		return nil, err
	}
	if err := s.RefreshCache(); err != nil {
		return nil, err
	}
	return policy, nil
}

func (s *service) Update(id string, input UpdateInput) (*AccessPolicy, error) {
	policy, err := s.repo.FindByID(id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
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
	if err := s.repo.Update(policy); err != nil {
		return nil, err
	}
	if err := s.RefreshCache(); err != nil {
		return nil, err
	}
	return policy, nil
}

func (s *service) Disable(id string) (*AccessPolicy, error) {
	policy, err := s.repo.FindByID(id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
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
	if err := s.repo.Update(policy); err != nil {
		return nil, err
	}
	if err := s.RefreshCache(); err != nil {
		return nil, err
	}
	return policy, nil
}

func (s *service) RefreshCache() error {
	if s.cache == nil {
		return nil
	}
	policies, err := s.repo.ListActive(time.Now())
	if err != nil {
		_ = s.cache.DeleteValue(utils.ToPolicySnapshotCacheKey())
		return fmt.Errorf("%w: refresh policy snapshot: %v", security.ErrPolicyUnavailable, err)
	}
	if err := s.cache.SetValue(utils.ToPolicySnapshotCacheKey(), policies, policyCacheTTL); err != nil {
		_ = s.cache.DeleteValue(utils.ToPolicySnapshotCacheKey())
		return fmt.Errorf("%w: write policy snapshot: %v", security.ErrPolicyUnavailable, err)
	}
	return nil
}

func (s *service) loadSnapshot() ([]*AccessPolicy, error) {
	var policies []*AccessPolicy
	if s.cache != nil {
		if err := s.cache.GetValue(utils.ToPolicySnapshotCacheKey(), &policies); err == nil {
			return policies, nil
		}
	}
	policies, err := s.repo.ListActive(time.Now())
	if err != nil {
		return nil, fmt.Errorf("%w: load policy snapshot: %v", security.ErrPolicyUnavailable, err)
	}
	if s.cache != nil {
		if err := s.cache.SetValue(utils.ToPolicySnapshotCacheKey(), policies, policyCacheTTL); err != nil {
			s.log.Warn("Unable to warm access policy cache", zap.Error(err))
		}
	}
	return policies, nil
}

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
		if err != nil || parsed.Address != input.Principal || len(input.Principal) > 320 || security.IsStudentEmail(input.Principal) && input.Kind == KindAllowlist {
			return CreateInput{}, ErrInvalidPolicy
		}
	} else {
		input.Principal = security.NormalizeGoogleSubject(input.Principal)
		if input.Principal == "" || strings.IndexFunc(input.Principal, func(r rune) bool { return r == ' ' || r == '\t' || r == '\r' || r == '\n' }) >= 0 || len(input.Principal) > 320 {
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
		if activeAt(policy, now) && policy.Kind == KindAllowlist && policy.PrincipalType == PrincipalEmail && policy.Principal == email {
			return true
		}
	}
	return false
}

func activeAt(policy *AccessPolicy, now time.Time) bool {
	return policy != nil && policy.Enabled && (policy.ExpiresAt == nil || policy.ExpiresAt.After(now))
}
