package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/esc-chula/intania-888-backend/internal/identity"
	"github.com/esc-chula/intania-888-backend/internal/security"
	"github.com/esc-chula/intania-888-backend/internal/value"
	"github.com/esc-chula/intania-888-backend/pkg/config"

	"github.com/redis/go-redis/v9"
	"golang.org/x/oauth2"

	oauthpkg "github.com/esc-chula/intania-888-backend/pkg/oauth"
)

type authTestConfig struct {
	server config.Server
	jwt    config.JWT
	oauth  config.OAuth
}

func (c authTestConfig) GetServer() config.Server {
	return c.server
}

func (c authTestConfig) GetDB() config.DB {
	return config.DB{}
}

func (c authTestConfig) GetCache() config.Cache {
	return config.Cache{}
}

func (c authTestConfig) GetJWT() config.JWT {
	return c.jwt
}

func (c authTestConfig) GetOAuth() config.OAuth {
	return c.oauth
}

func (c authTestConfig) GetSession() config.Session {
	return config.Session{
		IdleTTLSeconds:     config.DefaultSessionIdleTTLSeconds,
		AbsoluteTTLSeconds: config.DefaultSessionAbsoluteTTLSeconds,
	}
}

func (c authTestConfig) GetSwagger() config.Swagger {
	return config.Swagger{}
}

func (c authTestConfig) GetCORS() config.CORS {
	return config.CORS{}
}

func (c authTestConfig) GetDailyReward() config.DailyReward {
	return config.DailyReward{}
}

func (c authTestConfig) GetTeamCoin() config.TeamCoin {
	return config.TeamCoin{}
}

type memoryAuthRepository struct {
	mu      sync.Mutex
	values  map[string][]byte
	ttls    map[string]int
	expires map[string]time.Time
}

func newMemoryAuthRepository() *memoryAuthRepository {
	return &memoryAuthRepository{
		values:  make(map[string][]byte),
		ttls:    make(map[string]int),
		expires: make(map[string]time.Time),
	}
}

func (r *memoryAuthRepository) SetCacheValue(key string, value interface{}, ttl int) error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.values[key] = encoded
	r.ttls[key] = ttl
	if ttl > 0 {
		r.expires[key] = time.Now().Add(time.Duration(ttl) * time.Second)
	}

	return nil
}

func (r *memoryAuthRepository) GetCacheValue(key string, value interface{}) error {
	r.mu.Lock()
	if expiresAt, ok := r.expires[key]; ok && !time.Now().Before(expiresAt) {
		delete(r.values, key)
		delete(r.ttls, key)
		delete(r.expires, key)
	}
	encoded, ok := r.values[key]
	if ok {
		encoded = append([]byte(nil), encoded...)
	}
	r.mu.Unlock()
	if !ok {
		return redis.Nil
	}

	return json.Unmarshal(encoded, value)
}

func (r *memoryAuthRepository) DeleteCacheValue(key string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.values[key]; !ok {
		return redis.Nil
	}
	delete(r.values, key)
	delete(r.ttls, key)
	delete(r.expires, key)

	return nil
}

func (r *memoryAuthRepository) DeleteCacheValues(keys ...string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, key := range keys {
		delete(r.values, key)
		delete(r.ttls, key)
		delete(r.expires, key)
	}

	return nil
}

func (r *memoryAuthRepository) ConsumeCacheValue(key string, value interface{}) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if expiresAt, ok := r.expires[key]; ok && !time.Now().Before(expiresAt) {
		delete(r.values, key)
		delete(r.ttls, key)
		delete(r.expires, key)
	}
	encoded, ok := r.values[key]
	if !ok {
		return redis.Nil
	}
	delete(r.values, key)
	delete(r.ttls, key)
	delete(r.expires, key)

	return json.Unmarshal(encoded, value)
}

func (r *memoryAuthRepository) CompareAndSwapCacheValues(expected map[string]interface{}, replacements map[string]interface{}, ttl int) (bool, error) {
	encodedExpected := make(map[string][]byte, len(expected))
	for key, value := range expected {
		encoded, err := json.Marshal(value)
		if err != nil {
			return false, err
		}
		encodedExpected[key] = encoded
	}
	encodedReplacements := make(map[string][]byte, len(replacements))
	for key, value := range replacements {
		encoded, err := json.Marshal(value)
		if err != nil {
			return false, err
		}
		encodedReplacements[key] = encoded
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	for key, expectedValue := range encodedExpected {
		actual, ok := r.values[key]
		if !ok || string(actual) != string(expectedValue) {
			return false, nil
		}
	}
	for key, value := range encodedReplacements {
		r.values[key] = value
		r.ttls[key] = ttl
		if ttl > 0 {
			r.expires[key] = time.Now().Add(time.Duration(ttl) * time.Second)
		}
	}

	return true, nil
}

type memoryUserRepository struct {
	users       map[string]*identity.User
	createCount int
}

func newMemoryUserRepository() *memoryUserRepository {
	return &memoryUserRepository{users: make(map[string]*identity.User)}
}

func (r *memoryUserRepository) Create(ctx context.Context, user *identity.User) error {
	r.users[user.Email] = user
	r.createCount++

	return nil
}

func (r *memoryUserRepository) GetByID(ctx context.Context, id string) (*identity.User, error) {
	for _, user := range r.users {
		if user.ID == id {
			return user, nil
		}
	}

	return nil, identity.ErrUserNotFound
}

func (r *memoryUserRepository) GetByEmail(ctx context.Context, email string) (*identity.User, error) {
	user, ok := r.users[email]
	if !ok {
		return nil, identity.ErrUserNotFound
	}

	return user, nil
}

func (r *memoryUserRepository) GetAll() ([]*identity.User, error) {
	return nil, nil
}

func (r *memoryUserRepository) Update(*identity.User) error {
	return nil
}

func (r *memoryUserRepository) DeductCoin(string, value.Money) (value.Money, error) {
	return value.Money{}, nil
}

type fakeGoogleOAuthClient struct {
	config   *oauth2.Config
	info     *oauthpkg.GoogleUserInfo
	err      error
	verifier *string
}

type testPolicyChecker struct {
	blacklisted bool
	err         error
}

func (c testPolicyChecker) EvaluateLogin(context.Context, string, string, string) (security.PolicyDecision, error) {
	return security.PolicyDecision{Allowed: true}, c.err
}

func (c testPolicyChecker) IsBlacklisted(context.Context, string, string) (bool, error) {
	return c.blacklisted, c.err
}

func (c fakeGoogleOAuthClient) GetUserInfo(ctx context.Context, _, codeVerifier string) (*oauthpkg.GoogleUserInfo, error) {
	if c.verifier != nil {
		*c.verifier = codeVerifier
	}

	return c.info, c.err
}

func (c fakeGoogleOAuthClient) OAuthConfig() *oauth2.Config {
	return c.config
}

func newAuthTestService(info *oauthpkg.GoogleUserInfo) (*Service, *memoryAuthRepository, *memoryUserRepository) {
	repo := newMemoryAuthRepository()
	users := newMemoryUserRepository()
	cfg := authTestConfig{
		server: config.Server{
			Name: "intania-test",
			Env:  "development",
		},
		jwt: config.JWT{
			AccessTokenSecret: "access-secret",
		},
		oauth: config.OAuth{
			Registry: &config.AuthRegistry{
				Lifetimes: config.AuthLifetimes{
					Login:  120,
					Access: 3600,
				},
			},
		},
	}
	verifier := new(string)
	client := &fakeGoogleOAuthClient{
		config: &oauth2.Config{
			ClientID:    "client-id",
			RedirectURL: "https://api.example.test/api/v1/auth/callback",
			Endpoint: oauth2.Endpoint{
				AuthURL: "https://accounts.example.test/oauth/authorize",
			},
			Scopes: []string{"openid", "email"},
		},
		info:     info,
		verifier: verifier,
	}
	service := NewService(repo, users, cfg, client, testPolicyChecker{})

	return service, repo, users
}

func createOAuthState(t *testing.T, service *Service) string {
	t.Helper()
	login, err := service.StartOAuthLogin(context.Background())
	if err != nil {
		t.Fatalf("StartOAuthLogin() error = %v", err)
	}
	parsed, err := url.Parse(login.URL)
	if err != nil {
		t.Fatalf("Parse login URL: %v", err)
	}
	if parsed.Query().Get("redirect_to") != "" {
		t.Fatal("OAuth URL contains caller-controlled redirect_to")
	}
	if parsed.Query().Get("state") != login.State {
		t.Fatal("OAuth URL state does not match returned state")
	}
	if parsed.Query().Get("code_challenge") == "" || parsed.Query().Get("code_challenge_method") != "S256" {
		t.Fatalf("OAuth URL is missing S256 PKCE parameters: %q", parsed.Query())
	}
	if parsed.Query().Get("code_verifier") != "" {
		t.Fatal("OAuth URL contains the PKCE verifier")
	}
	var stateRecord oauthStateRecord
	if err := service.authRepo.(*memoryAuthRepository).GetCacheValue(security.ToOAuthStateCacheKey(login.State), &stateRecord); err != nil {
		t.Fatalf("OAuth state lookup error: %v", err)
	}
	if stateRecord.CodeVerifier == "" {
		t.Fatal("OAuth state did not store a PKCE verifier")
	}

	return login.State
}

func TestOAuthStateIsBoundAndConsumedOnce(t *testing.T) {
	service, repo, _ := newAuthTestService(&oauthpkg.GoogleUserInfo{})
	state := createOAuthState(t, service)
	if got := repo.ttls[security.ToOAuthStateCacheKey(state)]; got != 120 {
		t.Fatalf("state TTL = %d, want 120", got)
	}

	if _, err := service.VerifyOAuthLogin(context.Background(), "code", state, "different", ""); !errors.Is(err, ErrInvalidOAuthState) {
		t.Fatalf("mismatched state error = %v, want ErrInvalidOAuthState", err)
	}
	if _, err := service.VerifyOAuthLogin(context.Background(), "code", state, state, ""); !errors.Is(err, ErrUnverifiedEmail) {
		t.Fatalf("unverified email error = %v, want ErrUnverifiedEmail", err)
	}
	if client, ok := service.oauthClient.(*fakeGoogleOAuthClient); !ok || client.verifier == nil || *client.verifier == "" {
		t.Fatal("OAuth code exchange did not receive the server-side PKCE verifier")
	}
	if _, err := service.VerifyOAuthLogin(context.Background(), "code", state, state, ""); !errors.Is(err, ErrInvalidOAuthState) {
		t.Fatalf("replayed state error = %v, want ErrInvalidOAuthState", err)
	}
}

func TestExpiredOAuthStateCannotBeConsumed(t *testing.T) {
	service, repo, _ := newAuthTestService(&oauthpkg.GoogleUserInfo{
		Email:         "student@student.chula.ac.th",
		VerifiedEmail: true,
	})
	state := createOAuthState(t, service)
	repo.mu.Lock()
	repo.expires[security.ToOAuthStateCacheKey(state)] = time.Now().Add(-time.Second)
	repo.mu.Unlock()

	if _, err := service.VerifyOAuthLogin(context.Background(), "code", state, state, ""); !errors.Is(err, ErrInvalidOAuthState) {
		t.Fatalf("expired OAuth state error = %v, want ErrInvalidOAuthState", err)
	}
}

func (r *memoryAuthRepository) RotateSession(ctx context.Context,
	userKey, sessionKey, previousKey string,
	state security.Session,
	idleTTLSeconds, absoluteTTLSeconds int,
) error {
	r.mu.Lock()
	old := string(r.values[userKey])
	r.mu.Unlock()
	if err := r.SetCacheValue(sessionKey, sessionRecord{
		UserID:    state.UserID,
		CreatedAt: state.CreatedAt,
		ExpiresAt: state.ExpiresAt,
		CSRFToken: state.CSRFToken,
	}, idleTTLSeconds); err != nil {
		return err
	}
	if err := r.SetCacheValue(userKey, sessionKey, absoluteTTLSeconds); err != nil {
		return err
	}
	if previousKey != security.ToSessionCacheKey("") {
		if err := r.DeleteSession(context.Background(), previousKey); err != nil {
			return err
		}
	}
	if old != "" {
		var previous string
		if err := json.Unmarshal([]byte(old), &previous); err != nil {
			return err
		}
		if err := r.DeleteSession(context.Background(), previous); err != nil {
			return err
		}
	}

	return nil
}

func (r *memoryAuthRepository) DeleteSession(ctx context.Context, key string) error {
	return r.DeleteCacheValues(key)
}

func TestOpaqueSessionRotationAndFailedLogin(t *testing.T) {
	info := &oauthpkg.GoogleUserInfo{
		ID:            "google-id",
		Email:         "student@student.chula.ac.th",
		VerifiedEmail: true,
	}
	service, repo, _ := newAuthTestService(info)
	state := createOAuthState(t, service)
	first, err := service.VerifyOAuthLogin(context.Background(), "code", state, state, "")
	if err != nil {
		t.Fatal(err)
	}
	if !first.IsNewUser || first.SessionID == "" {
		t.Fatal("new login missing opaque session")
	}
	var record sessionRecord
	if err := repo.GetCacheValue(security.ToSessionCacheKey(first.SessionID), &record); err != nil {
		t.Fatal(err)
	}
	if record.UserID != "google-id" || record.CSRFToken == "" || record.ExpiresAt-record.CreatedAt != 30*24*3600 {
		t.Fatalf("invalid session: %+v", record)
	}
	if _, err := service.VerifyOAuthLogin(context.Background(), "code", state, state, ""); !errors.Is(err, ErrInvalidOAuthState) {
		t.Fatalf("reused state: %v", err)
	}
	client := service.oauthClient.(*fakeGoogleOAuthClient)
	client.err = errors.New("Google failed")
	badState := createOAuthState(t, service)
	if _, err := service.VerifyOAuthLogin(context.Background(), "code", badState, badState, first.SessionID); err == nil {
		t.Fatal("expected failed login")
	}
	if err := repo.GetCacheValue(security.ToSessionCacheKey(first.SessionID), &record); err != nil {
		t.Fatal("failed login revoked previous session")
	}
	client.err = nil
	secondState := createOAuthState(t, service)
	second, err := service.VerifyOAuthLogin(context.Background(), "code", secondState, secondState, first.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	if second.IsNewUser || second.SessionID == first.SessionID {
		t.Fatal("session was not rotated")
	}
	if err := repo.GetCacheValue(security.ToSessionCacheKey(first.SessionID), &record); err == nil {
		t.Fatal("old session still exists")
	}
	if err := service.Logout(context.Background(), second.SessionID); err != nil {
		t.Fatal(err)
	}
	if err := service.Logout(context.Background(), second.SessionID); err != nil {
		t.Fatal("logout is not idempotent")
	}
}

func (r *memoryAuthRepository) StoreOAuthState(ctx context.Context, key string, state OAuthState, ttl int) error {
	return r.SetCacheValue(key, oauthStateRecord(state), ttl)
}

func (r *memoryAuthRepository) ConsumeOAuthState(ctx context.Context, key string) (OAuthState, error) {
	var record oauthStateRecord
	if err := r.ConsumeCacheValue(key, &record); err != nil {
		if errors.Is(err, redis.Nil) {
			return OAuthState{}, errors.Join(ErrInvalidOAuthState, err)
		}

		return OAuthState{}, err
	}

	return OAuthState(record), nil
}
