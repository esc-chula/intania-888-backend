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
	"github.com/google/uuid"
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
		return errors.New("cache miss")
	}
	return json.Unmarshal(encoded, value)
}

func (r *memoryAuthRepository) DeleteCacheValue(key string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.values[key]; !ok {
		return errors.New("cache miss")
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
		return errors.New("cache miss")
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
			AccessTokenSecret:      "access-secret",
			AccessTokenExpiration:  300,
			RefreshTokenExpiration: 3600,
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
	service := NewAuthService(repo, users, cfg, zap.NewNop(), client).(*authServiceImpl)
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

	if _, err := service.VerifyOAuthLogin("code", state, "different"); !errors.Is(err, ErrInvalidOAuthState) {
		t.Fatalf("mismatched state error = %v, want ErrInvalidOAuthState", err)
	}
	if _, err := service.VerifyOAuthLogin("code", state, state); !errors.Is(err, ErrUnverifiedEmail) {
		t.Fatalf("unverified email error = %v, want ErrUnverifiedEmail", err)
	}
	if client, ok := service.oauthClient.(*fakeGoogleOAuthClient); !ok || client.verifier == nil || *client.verifier == "" {
		t.Fatal("OAuth code exchange did not receive the server-side PKCE verifier")
	}
	if _, err := service.VerifyOAuthLogin("code", state, state); !errors.Is(err, ErrInvalidOAuthState) {
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

	if _, err := service.VerifyOAuthLogin("code", state, state); !errors.Is(err, ErrInvalidOAuthState) {
		t.Fatalf("expired OAuth state error = %v, want ErrInvalidOAuthState", err)
	}
}

func TestConcurrentRefreshRotationAcceptsOnlyOneRequestAndRevokesOnReplay(t *testing.T) {
	service, repo, users := newAuthTestService(&oauthpkg.GoogleUserInfo{
		Id:            uuid.NewString(),
		Email:         "student@student.chula.ac.th",
		Name:          "Student",
		VerifiedEmail: true,
	})
	users.users["student@student.chula.ac.th"] = &model.User{Id: "user-id", Email: "student@student.chula.ac.th", RoleId: "USER"}
	state := createOAuthState(t, service)
	credentials, err := service.VerifyOAuthLogin("code", state, state)
	if err != nil {
		t.Fatalf("VerifyOAuthLogin() error = %v", err)
	}

	results := make(chan struct {
		credentials *SessionCredentials
		err         error
	}, 2)
	var wait sync.WaitGroup
	for range 2 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			rotated, refreshErr := service.RefreshToken(credentials.RefreshToken)
			results <- struct {
				credentials *SessionCredentials
				err         error
			}{rotated, refreshErr}
		}()
	}
	wait.Wait()
	close(results)

	successes := 0
	replays := 0
	for result := range results {
		if result.err == nil {
			successes++
			continue
		}
		if errors.Is(result.err, ErrRefreshReplay) {
			replays++
		}
	}
	if successes != 1 || replays != 1 {
		t.Fatalf("concurrent refresh results = successes %d, replays %d; want one of each", successes, replays)
	}

	claims, err := utils.JwtParseAccessToken(credentials.AccessToken, "access-secret", "intania-test", "intania-test")
	if err != nil {
		t.Fatalf("JwtParseAccessToken() error = %v", err)
	}
	var session model.SessionRecord
	if err := repo.GetCacheValue(utils.ToSessionCacheKey(claims.SessionId), &session); err == nil {
		t.Fatal("session remains active after concurrent refresh replay")
	}
}

func TestRefreshRotationAcceptsRoleChanges(t *testing.T) {
	tests := []struct {
		name    string
		oldRole string
		newRole string
	}{
		{name: "promote user", oldRole: security.RoleUser, newRole: security.RoleAdmin},
		{name: "demote admin", oldRole: security.RoleAdmin, newRole: security.RoleUser},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			userID := uuid.NewString()
			service, repo, users := newAuthTestService(&oauthpkg.GoogleUserInfo{
				Id:            userID,
				Email:         "student@student.chula.ac.th",
				Name:          "Student",
				VerifiedEmail: true,
			})
			users.users["student@student.chula.ac.th"] = &model.User{
				Id:     userID,
				Email:  "student@student.chula.ac.th",
				RoleId: test.oldRole,
			}

			state := createOAuthState(t, service)
			credentials, err := service.VerifyOAuthLogin("code", state, state)
			if err != nil {
				t.Fatalf("VerifyOAuthLogin() error = %v", err)
			}

			users.users["student@student.chula.ac.th"].RoleId = test.newRole
			rotated, err := service.RefreshToken(credentials.RefreshToken)
			if err != nil {
				t.Fatalf("RefreshToken() after role change error = %v", err)
			}
			if rotated.RefreshToken == credentials.RefreshToken {
				t.Fatal("refresh token was not rotated")
			}

			claims, err := utils.JwtParseAccessToken(rotated.AccessToken, "access-secret", "intania-test", "intania-test")
			if err != nil {
				t.Fatalf("JwtParseAccessToken() error = %v", err)
			}
			if claims.Role != test.newRole {
				t.Fatalf("access token role = %q, want %q", claims.Role, test.newRole)
			}

			var session model.SessionRecord
			if err := repo.GetCacheValue(utils.ToSessionCacheKey(claims.SessionId), &session); err != nil {
				t.Fatalf("session lookup error = %v", err)
			}
			if session.Role != test.newRole {
				t.Fatalf("stored session role = %q, want %q", session.Role, test.newRole)
			}
		})
	}
}

func TestOAuthRequiresVerifiedAllowlistedEmailAndCreatesHashedSession(t *testing.T) {
	service, repo, users := newAuthTestService(&oauthpkg.GoogleUserInfo{
		Id:            uuid.NewString(),
		Email:         "6630000000@STUDENT.CHULA.AC.TH",
		Name:          "Student",
		VerifiedEmail: true,
	})
	state := createOAuthState(t, service)
	credentials, err := service.VerifyOAuthLogin("code", state, state)
	if err != nil {
		t.Fatalf("VerifyOAuthLogin() error = %v", err)
	}
	if !credentials.IsNewUser {
		t.Fatal("new user result has IsNewUser=false")
	}
	if users.createCount != 1 {
		t.Fatalf("created users = %d, want 1", users.createCount)
	}
	if credentials.AccessToken == "" || credentials.RefreshToken == "" {
		t.Fatal("session credentials are empty")
	}

	claims, err := utils.JwtParseAccessToken(credentials.AccessToken, "access-secret", "intania-test", "intania-test")
	if err != nil {
		t.Fatalf("JwtParseAccessToken() error = %v", err)
	}
	var session model.SessionRecord
	if err := repo.GetCacheValue(utils.ToSessionCacheKey(claims.SessionId), &session); err != nil {
		t.Fatalf("session lookup error = %v", err)
	}
	if session.RefreshTokenHash != utils.HashOpaqueToken(credentials.RefreshToken) {
		t.Fatal("session does not contain refresh token digest")
	}
	if string(repo.values[utils.ToSessionCacheKey(claims.SessionId)]) == credentials.RefreshToken {
		t.Fatal("raw refresh token was stored in session")
	}
}

func TestExistingAdminIsImplicitlyAllowlisted(t *testing.T) {
	userID := uuid.NewString()
	service, _, users := newAuthTestService(&oauthpkg.GoogleUserInfo{
		Id:            userID,
		Email:         "admin@example.com",
		Name:          "Admin",
		VerifiedEmail: true,
	})
	users.users["admin@example.com"] = &model.User{Id: userID, Email: "admin@example.com", RoleId: "ADMIN"}

	state := createOAuthState(t, service)
	credentials, err := service.VerifyOAuthLogin("code", state, state)
	if err != nil {
		t.Fatalf("VerifyOAuthLogin() error = %v", err)
	}
	if credentials.IsNewUser {
		t.Fatal("existing admin was treated as a new user")
	}
}

func TestRefreshRejectsAndRevokesBlacklistedUser(t *testing.T) {
	service, repo, users := newAuthTestService(&oauthpkg.GoogleUserInfo{
		Id:            uuid.NewString(),
		Email:         "student@student.chula.ac.th",
		Name:          "Student",
		VerifiedEmail: true,
	})
	users.users["student@student.chula.ac.th"] = &model.User{Id: "user-id", Email: "student@student.chula.ac.th", RoleId: "USER"}
	state := createOAuthState(t, service)
	credentials, err := service.VerifyOAuthLogin("code", state, state)
	if err != nil {
		t.Fatalf("VerifyOAuthLogin() error = %v", err)
	}

	service.policy = testPolicyChecker{blacklisted: true}
	if _, err := service.RefreshToken(credentials.RefreshToken); !errors.Is(err, ErrEmailNotAllowed) {
		t.Fatalf("blacklisted refresh error = %v, want ErrEmailNotAllowed", err)
	}
	claims, err := utils.JwtParseAccessToken(credentials.AccessToken, "access-secret", "intania-test", "intania-test")
	if err != nil {
		t.Fatalf("JwtParseAccessToken() error = %v", err)
	}
	var session model.SessionRecord
	if err := repo.GetCacheValue(utils.ToSessionCacheKey(claims.SessionId), &session); err == nil {
		t.Fatal("blacklisted user's session remains active")
	}
}

func TestExistingUserDoesNotSetNewUserFlagAndRefreshReplayRevokesSession(t *testing.T) {
	userID := uuid.NewString()
	service, repo, users := newAuthTestService(&oauthpkg.GoogleUserInfo{
		Id:            userID,
		Email:         "student@student.chula.ac.th",
		Name:          "Student",
		VerifiedEmail: true,
	})
	users.users["student@student.chula.ac.th"] = &model.User{Id: userID, Email: "student@student.chula.ac.th", RoleId: "USER"}

	state := createOAuthState(t, service)
	credentials, err := service.VerifyOAuthLogin("code", state, state)
	if err != nil {
		t.Fatalf("VerifyOAuthLogin() error = %v", err)
	}
	if credentials.IsNewUser {
		t.Fatal("existing user result has IsNewUser=true")
	}

	claims, err := utils.JwtParseAccessToken(credentials.AccessToken, "access-secret", "intania-test", "intania-test")
	if err != nil {
		t.Fatalf("JwtParseAccessToken() error = %v", err)
	}
	rotated, err := service.RefreshToken(credentials.RefreshToken)
	if err != nil {
		t.Fatalf("RefreshToken() error = %v", err)
	}
	if rotated.RefreshToken == credentials.RefreshToken {
		t.Fatal("refresh token was not rotated")
	}
	if _, err := service.RefreshToken(credentials.RefreshToken); !errors.Is(err, ErrRefreshReplay) {
		t.Fatalf("replayed refresh error = %v, want ErrRefreshReplay", err)
	}
	var session model.SessionRecord
	if err := repo.GetCacheValue(utils.ToSessionCacheKey(claims.SessionId), &session); err == nil {
		t.Fatal("session remains active after refresh replay")
	}
	if _, err := service.RefreshToken(rotated.RefreshToken); !errors.Is(err, ErrInvalidRefresh) {
		t.Fatalf("rotated token after revocation error = %v, want ErrInvalidRefresh", err)
	}
}
