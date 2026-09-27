package auth

import (
	"encoding/json"
	"errors"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/esc-chula/intania-888-backend/internal/model"
	"github.com/esc-chula/intania-888-backend/internal/security"
	"github.com/esc-chula/intania-888-backend/pkg/config"
	oauthpkg "github.com/esc-chula/intania-888-backend/pkg/oauth"
	"github.com/esc-chula/intania-888-backend/utils"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
	"gorm.io/gorm"
)

type authTestConfig struct {
	server config.Server
	jwt    config.Jwt
	oauth  config.OAuth
}

func (c authTestConfig) GetServer() config.Server { return c.server }
func (c authTestConfig) GetDb() config.Db         { return config.Db{} }
func (c authTestConfig) GetCache() config.Cache   { return config.Cache{} }
func (c authTestConfig) GetJwt() config.Jwt       { return c.jwt }
func (c authTestConfig) GetOAuth() config.OAuth   { return c.oauth }
func (c authTestConfig) GetSession() config.Session {
	return config.Session{
		IdleTTLSeconds:     config.DefaultSessionIdleTTLSeconds,
		AbsoluteTTLSeconds: config.DefaultSessionAbsoluteTTLSeconds,
	}
}
func (c authTestConfig) GetSwagger() config.Swagger {
	return config.Swagger{}
}
func (c authTestConfig) GetCors() config.Cors { return config.Cors{} }

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
	users       map[string]*model.User
	createCount int
}

func newMemoryUserRepository() *memoryUserRepository {
	return &memoryUserRepository{users: make(map[string]*model.User)}
}

func (r *memoryUserRepository) Create(user *model.User) error {
	r.users[user.Email] = user
	r.createCount++
	return nil
}

func (r *memoryUserRepository) GetById(id string) (*model.User, error) {
	for _, user := range r.users {
		if user.Id == id {
			return user, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (r *memoryUserRepository) GetByEmail(email string) (*model.User, error) {
	user, ok := r.users[email]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return user, nil
}

func (r *memoryUserRepository) GetAll() ([]*model.User, error) { return nil, nil }
func (r *memoryUserRepository) Update(*model.User) error       { return nil }

func (r *memoryUserRepository) DeductCoin(string, model.Money) (model.Money, error) {
	return model.Money{}, nil
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

func (c testPolicyChecker) EvaluateLogin(string, string, string) (security.PolicyDecision, error) {
	return security.PolicyDecision{Allowed: true}, c.err
}

func (c testPolicyChecker) IsBlacklisted(string, string) (bool, error) {
	return c.blacklisted, c.err
}

func (c fakeGoogleOAuthClient) GetUserInfo(_, codeVerifier string) (*oauthpkg.GoogleUserInfo, error) {
	if c.verifier != nil {
		*c.verifier = codeVerifier
	}
	return c.info, c.err
}

func (c fakeGoogleOAuthClient) OAuthConfig() *oauth2.Config { return c.config }

func newAuthTestService(info *oauthpkg.GoogleUserInfo) (*authServiceImpl, *memoryAuthRepository, *memoryUserRepository) {
	repo := newMemoryAuthRepository()
	users := newMemoryUserRepository()
	cfg := authTestConfig{
		server: config.Server{Name: "intania-test", Env: "development"},
		jwt: config.Jwt{
			AccessTokenSecret: "access-secret",
		},
		oauth: config.OAuth{
			StateExpiration:      120,
			PostLoginRedirectUrl: "https://frontend.example.test/after-login?source=oauth",
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
	service := NewAuthService(repo, users, cfg, zap.NewNop(), client, testPolicyChecker{}).(*authServiceImpl)
	return service, repo, users
}

func createOAuthState(t *testing.T, service *authServiceImpl) string {
	t.Helper()
	login, err := service.StartOAuthLogin()
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
	var stateRecord model.OAuthStateRecord
	if err := service.authRepo.GetCacheValue(utils.ToOAuthStateCacheKey(login.State), &stateRecord); err != nil {
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
	if got := repo.ttls[utils.ToOAuthStateCacheKey(state)]; got != 120 {
		t.Fatalf("state TTL = %d, want 120", got)
	}

	if _, err := service.VerifyOAuthLogin("code", state, "different", ""); !errors.Is(err, ErrInvalidOAuthState) {
		t.Fatalf("mismatched state error = %v, want ErrInvalidOAuthState", err)
	}
	if _, err := service.VerifyOAuthLogin("code", state, state, ""); !errors.Is(err, ErrUnverifiedEmail) {
		t.Fatalf("unverified email error = %v, want ErrUnverifiedEmail", err)
	}
	if client, ok := service.oauthClient.(*fakeGoogleOAuthClient); !ok || client.verifier == nil || *client.verifier == "" {
		t.Fatal("OAuth code exchange did not receive the server-side PKCE verifier")
	}
	if _, err := service.VerifyOAuthLogin("code", state, state, ""); !errors.Is(err, ErrInvalidOAuthState) {
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
	repo.expires[utils.ToOAuthStateCacheKey(state)] = time.Now().Add(-time.Second)
	repo.mu.Unlock()

	if _, err := service.VerifyOAuthLogin("code", state, state, ""); !errors.Is(err, ErrInvalidOAuthState) {
		t.Fatalf("expired OAuth state error = %v, want ErrInvalidOAuthState", err)
	}
}

func (r *memoryAuthRepository) RotateSession(
	userKey, sessionKey, previousKey string,
	value interface{},
	idleTTLSeconds, absoluteTTLSeconds int,
) error {
	r.mu.Lock()
	old := string(r.values[userKey])
	r.mu.Unlock()
	if err := r.SetCacheValue(sessionKey, value, idleTTLSeconds); err != nil {
		return err
	}
	if err := r.SetCacheValue(userKey, sessionKey, absoluteTTLSeconds); err != nil {
		return err
	}
	if previousKey != utils.ToSessionCacheKey("") {
		_ = r.DeleteSession(previousKey)
	}
	if old != "" {
		var previous string
		_ = json.Unmarshal([]byte(old), &previous)
		_ = r.DeleteSession(previous)
	}
	return nil
}
func (r *memoryAuthRepository) DeleteSession(key string) error { return r.DeleteCacheValues(key) }

func TestOpaqueSessionRotationAndFailedLogin(t *testing.T) {
	info := &oauthpkg.GoogleUserInfo{Id: "google-id", Email: "student@student.chula.ac.th", VerifiedEmail: true}
	service, repo, _ := newAuthTestService(info)
	state := createOAuthState(t, service)
	first, err := service.VerifyOAuthLogin("code", state, state, "")
	if err != nil {
		t.Fatal(err)
	}
	if !first.IsNewUser || first.SessionID == "" {
		t.Fatal("new login missing opaque session")
	}
	var record model.SessionRecord
	if err := repo.GetCacheValue(utils.ToSessionCacheKey(first.SessionID), &record); err != nil {
		t.Fatal(err)
	}
	if record.UserId != "google-id" || record.CSRFToken == "" || record.ExpiresAt-record.CreatedAt != 30*24*3600 {
		t.Fatalf("invalid session: %+v", record)
	}
	if _, err := service.VerifyOAuthLogin("code", state, state, ""); !errors.Is(err, ErrInvalidOAuthState) {
		t.Fatalf("reused state: %v", err)
	}
	client := service.oauthClient.(*fakeGoogleOAuthClient)
	client.err = errors.New("Google failed")
	badState := createOAuthState(t, service)
	if _, err := service.VerifyOAuthLogin("code", badState, badState, first.SessionID); err == nil {
		t.Fatal("expected failed login")
	}
	if err := repo.GetCacheValue(utils.ToSessionCacheKey(first.SessionID), &record); err != nil {
		t.Fatal("failed login revoked previous session")
	}
	client.err = nil
	secondState := createOAuthState(t, service)
	second, err := service.VerifyOAuthLogin("code", secondState, secondState, first.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	if second.IsNewUser || second.SessionID == first.SessionID {
		t.Fatal("session was not rotated")
	}
	if err := repo.GetCacheValue(utils.ToSessionCacheKey(first.SessionID), &record); err == nil {
		t.Fatal("old session still exists")
	}
	if err := service.Logout(second.SessionID); err != nil {
		t.Fatal(err)
	}
	if err := service.Logout(second.SessionID); err != nil {
		t.Fatal("logout is not idempotent")
	}
}

func TestExternalTokenIssuanceAndRevocationNeedNoAuditDatabase(t *testing.T) {
	service, repo, users := newAuthTestService(nil)
	if err := users.Create(&model.User{Id: "existing-user", Email: "existing@example.test"}); err != nil {
		t.Fatal(err)
	}
	token, id, err := service.IssueExternalToken("existing-user")
	if err != nil {
		t.Fatal(err)
	}
	subject, tokenID, err := utils.JwtParseExternalToken(token, service.cfg.GetJwt().AccessTokenSecret, service.cfg.GetServer().Name)
	if err != nil || subject != "existing-user" || tokenID != id {
		t.Fatalf("external token claims = %q, %q, %v", subject, tokenID, err)
	}
	var record ExternalTokenRecord
	key := utils.ToExternalTokenCacheKey(id)
	if err := repo.GetCacheValue(key, &record); err != nil || record.SubjectID != subject {
		t.Fatalf("external token record = %+v, %v", record, err)
	}
	if err := service.RevokeExternalToken(id); err != nil {
		t.Fatal(err)
	}
	if err := repo.GetCacheValue(key, &record); err == nil {
		t.Fatal("revoked external token is still active")
	}
	if _, _, err := service.IssueExternalToken("missing-user"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("missing user error = %v", err)
	}
}
