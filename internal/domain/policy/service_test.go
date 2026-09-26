package policy

import (
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/esc-chula/intania-888-backend/internal/security"
	"github.com/esc-chula/intania-888-backend/utils"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type fakeRepository struct {
	mu       sync.Mutex
	active   []*AccessPolicy
	items    map[string]*AccessPolicy
	listCall int
	create   int
}

func newFakeRepository(policies ...*AccessPolicy) *fakeRepository {
	items := make(map[string]*AccessPolicy, len(policies))
	for _, policy := range policies {
		items[policy.ID] = policy
	}
	return &fakeRepository{active: policies, items: items}
}

func (r *fakeRepository) ListActive(time.Time) ([]*AccessPolicy, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.listCall++
	return append([]*AccessPolicy(nil), r.active...), nil
}

func (r *fakeRepository) List(filter ListFilter) (ListResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	items := make([]*AccessPolicy, 0, len(r.items))
	for _, policy := range r.items {
		if filter.Kind != "" && policy.Kind != filter.Kind {
			continue
		}
		if filter.PrincipalType != "" && policy.PrincipalType != filter.PrincipalType {
			continue
		}
		items = append(items, policy)
	}
	return ListResult{Items: items}, nil
}

func (r *fakeRepository) FindByID(id string) (*AccessPolicy, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	policy, ok := r.items[id]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return policy, nil
}

func (r *fakeRepository) FindByIdentity(kind, principalType, principal string) (*AccessPolicy, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, policy := range r.items {
		if policy.Kind == kind && policy.PrincipalType == principalType && policy.Principal == principal {
			return policy, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (r *fakeRepository) Create(policy *AccessPolicy) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.create++
	r.items[policy.ID] = policy
	r.active = append(r.active, policy)
	return nil
}

func (r *fakeRepository) Update(policy *AccessPolicy) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items[policy.ID] = policy
	for i, item := range r.active {
		if item.ID == policy.ID {
			r.active[i] = policy
		}
	}
	return nil
}

type fakeCache struct {
	mu     sync.Mutex
	values map[string][]byte
	sets   int
	getErr error
	setErr error
}

func newFakeCache() *fakeCache { return &fakeCache{values: make(map[string][]byte)} }

func (c *fakeCache) SetValue(key string, value interface{}, _ int) error {
	if c.setErr != nil {
		return c.setErr
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.values[key] = encoded
	c.sets++
	return nil
}

func (c *fakeCache) GetValue(key string, value interface{}) error {
	if c.getErr != nil {
		return c.getErr
	}
	c.mu.Lock()
	encoded, ok := c.values[key]
	c.mu.Unlock()
	if !ok {
		return errors.New("cache miss")
	}
	return json.Unmarshal(encoded, value)
}

func (c *fakeCache) DeleteValue(key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.values, key)
	return nil
}

func TestEvaluateLoginUsesAllowlistAndAdminImplicitAllow(t *testing.T) {
	repo := newFakeRepository(
		&AccessPolicy{ID: "allow", Kind: KindAllowlist, PrincipalType: PrincipalEmail, Principal: "partner@example.com", Enabled: true},
	)
	service := NewService(repo, newFakeCache(), zap.NewNop())

	decision, err := service.EvaluateLogin(" PARTNER@EXAMPLE.COM ", "subject", security.RoleUser)
	if err != nil {
		t.Fatalf("allowlisted login error = %v", err)
	}
	if !decision.Allowed || decision.Blacklisted {
		t.Fatalf("allowlisted decision = %+v, want allowed", decision)
	}

	decision, err = service.EvaluateLogin("admin@example.com", "admin-subject", security.RoleAdmin)
	if err != nil {
		t.Fatalf("admin login error = %v", err)
	}
	if !decision.Allowed || decision.Blacklisted {
		t.Fatalf("admin decision = %+v, want allowed", decision)
	}
}

func TestBlacklistOverridesAllowlistAndAdmin(t *testing.T) {
	repo := newFakeRepository(
		&AccessPolicy{ID: "allow", Kind: KindAllowlist, PrincipalType: PrincipalEmail, Principal: "admin@example.com", Enabled: true},
		&AccessPolicy{ID: "deny", Kind: KindBlacklist, PrincipalType: PrincipalEmail, Principal: "admin@example.com", Enabled: true},
	)
	service := NewService(repo, newFakeCache(), zap.NewNop())

	decision, err := service.EvaluateLogin("admin@example.com", "subject", security.RoleAdmin)
	if err != nil {
		t.Fatalf("blacklisted admin error = %v", err)
	}
	if decision.Allowed || !decision.Blacklisted {
		t.Fatalf("blacklisted admin decision = %+v, want denied and blacklisted", decision)
	}
}

func TestGoogleSubjectBlacklistAndStudentDomain(t *testing.T) {
	repo := newFakeRepository(
		&AccessPolicy{ID: "deny", Kind: KindBlacklist, PrincipalType: PrincipalGoogleSubject, Principal: "google-subject", Enabled: true},
	)
	service := NewService(repo, newFakeCache(), zap.NewNop())

	decision, err := service.EvaluateLogin("student@student.chula.ac.th", "other-subject", security.RoleUser)
	if err != nil || !decision.Allowed {
		t.Fatalf("student decision = %+v, error = %v; want allowed", decision, err)
	}
	decision, err = service.EvaluateLogin("student@student.chula.ac.th", "google-subject", security.RoleUser)
	if err != nil {
		t.Fatalf("subject blacklist error = %v", err)
	}
	if decision.Allowed || !decision.Blacklisted {
		t.Fatalf("subject blacklist decision = %+v, want denied", decision)
	}
}

func TestPolicyCacheMissFallsBackToRepositoryAndWarmsCache(t *testing.T) {
	repo := newFakeRepository(
		&AccessPolicy{ID: "deny", Kind: KindBlacklist, PrincipalType: PrincipalEmail, Principal: "blocked@example.com", Enabled: true},
	)
	cache := newFakeCache()
	service := NewService(repo, cache, zap.NewNop())

	if _, err := service.EvaluateLogin("blocked@example.com", "subject", security.RoleUser); err != nil {
		t.Fatalf("first policy evaluation error = %v", err)
	}
	if repo.listCall != 1 || cache.sets != 1 {
		t.Fatalf("repository calls = %d, cache sets = %d; want one each", repo.listCall, cache.sets)
	}
	if _, err := service.EvaluateLogin("blocked@example.com", "subject", security.RoleUser); err != nil {
		t.Fatalf("cached policy evaluation error = %v", err)
	}
	if repo.listCall != 1 {
		t.Fatalf("repository calls after cache hit = %d, want 1", repo.listCall)
	}
}

func TestPolicyFailsClosedWhenRepositoryUnavailable(t *testing.T) {
	service := NewService(&failingRepository{}, newFakeCache(), zap.NewNop())
	_, err := service.EvaluateLogin("partner@example.com", "subject", security.RoleUser)
	if !errors.Is(err, security.ErrPolicyUnavailable) {
		t.Fatalf("policy error = %v, want ErrPolicyUnavailable", err)
	}
}

func TestCreateRefreshesPolicyCache(t *testing.T) {
	repo := newFakeRepository()
	cache := newFakeCache()
	service := NewService(repo, cache, zap.NewNop())

	created, err := service.Create(CreateInput{
		Kind:          KindAllowlist,
		PrincipalType: PrincipalEmail,
		Principal:     "new@example.com",
		Reason:        "approved",
	})
	if err != nil {
		t.Fatalf("create policy error = %v", err)
	}
	if created.Principal != "new@example.com" || cache.sets != 1 {
		t.Fatalf("created policy = %+v, cache sets = %d", created, cache.sets)
	}
	var cached []*AccessPolicy
	if err := cache.GetValue(utils.ToPolicySnapshotCacheKey(), &cached); err != nil {
		t.Fatalf("read refreshed cache: %v", err)
	}
	if len(cached) != 1 || cached[0].ID != created.ID {
		t.Fatalf("cached policies = %+v, want created policy", cached)
	}
}

func TestPolicyMutationsSucceedWhenCacheRefreshFails(t *testing.T) {
	t.Run("create", func(t *testing.T) {
		repo := newFakeRepository()
		cache := newFakeCache()
		cache.setErr = errors.New("redis unavailable")
		service := NewService(repo, cache, zap.NewNop())

		created, err := service.Create(CreateInput{
			Kind:          KindAllowlist,
			PrincipalType: PrincipalEmail,
			Principal:     "new@example.com",
			Reason:        "approved",
		})
		if err != nil {
			t.Fatalf("create policy error = %v; database write should remain successful", err)
		}
		if _, err := repo.FindByID(created.ID); err != nil {
			t.Fatalf("persisted policy lookup error = %v", err)
		}
	})

	t.Run("update", func(t *testing.T) {
		repo := newFakeRepository(&AccessPolicy{
			ID:            "policy-id",
			Kind:          KindBlacklist,
			PrincipalType: PrincipalEmail,
			Principal:     "blocked@example.com",
			Reason:        "old reason",
			Enabled:       true,
		})
		cache := newFakeCache()
		cache.setErr = errors.New("redis unavailable")
		service := NewService(repo, cache, zap.NewNop())

		updated, err := service.Update("policy-id", UpdateInput{Reason: "new reason", ReasonSet: true})
		if err != nil {
			t.Fatalf("update policy error = %v; database write should remain successful", err)
		}
		if updated.Reason != "new reason" {
			t.Fatalf("updated reason = %q, want %q", updated.Reason, "new reason")
		}
	})

	t.Run("disable", func(t *testing.T) {
		repo := newFakeRepository(&AccessPolicy{
			ID:            "policy-id",
			Kind:          KindBlacklist,
			PrincipalType: PrincipalEmail,
			Principal:     "blocked@example.com",
			Reason:        "blocked",
			Enabled:       true,
		})
		cache := newFakeCache()
		cache.setErr = errors.New("redis unavailable")
		service := NewService(repo, cache, zap.NewNop())

		disabled, err := service.Disable("policy-id")
		if err != nil {
			t.Fatalf("disable policy error = %v; database write should remain successful", err)
		}
		if disabled.Enabled {
			t.Fatal("policy remains enabled after successful disable")
		}
	})
}

func TestValidateCreateInputRejectsInvalidPolicyCombinations(t *testing.T) {
	cases := []CreateInput{
		{Kind: KindAllowlist, PrincipalType: PrincipalGoogleSubject, Principal: "subject", Reason: "invalid"},
		{Kind: KindAllowlist, PrincipalType: PrincipalEmail, Principal: "student@student.chula.ac.th", Reason: "redundant"},
		{Kind: KindBlacklist, PrincipalType: PrincipalEmail, Principal: "bad", Reason: "invalid email"},
		{Kind: KindBlacklist, PrincipalType: PrincipalEmail, Principal: "blocked@example.com", Reason: ""},
	}
	for _, input := range cases {
		if _, err := ValidateCreateInput(input); !errors.Is(err, ErrInvalidPolicy) {
			t.Fatalf("ValidateCreateInput(%+v) error = %v, want ErrInvalidPolicy", input, err)
		}
	}
}

type failingRepository struct{}

func (*failingRepository) ListActive(time.Time) ([]*AccessPolicy, error) {
	return nil, errors.New("database unavailable")
}
func (*failingRepository) List(ListFilter) (ListResult, error) {
	return ListResult{}, errors.New("database unavailable")
}
func (*failingRepository) FindByID(string) (*AccessPolicy, error) {
	return nil, errors.New("database unavailable")
}
func (*failingRepository) FindByIdentity(string, string, string) (*AccessPolicy, error) {
	return nil, errors.New("database unavailable")
}
func (*failingRepository) Create(*AccessPolicy) error {
	return errors.New("database unavailable")
}
func (*failingRepository) Update(*AccessPolicy) error {
	return errors.New("database unavailable")
}
