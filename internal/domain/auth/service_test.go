package auth

import (
	"encoding/json"
	"errors"
	"net/url"
	"testing"

	"github.com/esc-chula/intania-888-backend/internal/model"
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
	values map[string][]byte
	ttls   map[string]int
}

func newMemoryAuthRepository() *memoryAuthRepository {
	return &memoryAuthRepository{
		values: make(map[string][]byte),
		ttls:   make(map[string]int),
	}
}

func (r *memoryAuthRepository) SetCacheValue(key string, value interface{}, ttl int) error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return err
	}
	r.values[key] = encoded
	r.ttls[key] = ttl
	return nil
}

func (r *memoryAuthRepository) GetCacheValue(key string, value interface{}) error {
	encoded, ok := r.values[key]
	if !ok {
		return errors.New("cache miss")
	}
	return json.Unmarshal(encoded, value)
}

func (r *memoryAuthRepository) DeleteCacheValue(key string) error {
	if _, ok := r.values[key]; !ok {
		return errors.New("cache miss")
	}
	delete(r.values, key)
	delete(r.ttls, key)
	return nil
}

func (r *memoryAuthRepository) ConsumeCacheValue(key string, value interface{}) error {
	encoded, ok := r.values[key]
	if !ok {
		return errors.New("cache miss")
	}
	delete(r.values, key)
	delete(r.ttls, key)
	return json.Unmarshal(encoded, value)
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
	config *oauth2.Config
	info   *oauthpkg.GoogleUserInfo
	err    error
}

func (c fakeGoogleOAuthClient) GetUserInfo(string) (*oauthpkg.GoogleUserInfo, error) {
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
	client := fakeGoogleOAuthClient{
		config: &oauth2.Config{
			ClientID:    "client-id",
			RedirectURL: "https://api.example.test/api/v1/auth/callback",
			Endpoint: oauth2.Endpoint{
				AuthURL: "https://accounts.example.test/oauth/authorize",
			},
			Scopes: []string{"openid", "email"},
		},
		info: info,
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
	if _, err := service.VerifyOAuthLogin("code", state, state); !errors.Is(err, ErrInvalidOAuthState) {
		t.Fatalf("replayed state error = %v, want ErrInvalidOAuthState", err)
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
