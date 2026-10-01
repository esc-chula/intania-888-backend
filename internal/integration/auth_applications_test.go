//go:build integration

package integration

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/oauth2"

	"github.com/esc-chula/intania-888-backend/internal/apierror"
	"github.com/esc-chula/intania-888-backend/internal/domain/auth"
	"github.com/esc-chula/intania-888-backend/internal/domain/middleware"
	"github.com/esc-chula/intania-888-backend/internal/identity"
	"github.com/esc-chula/intania-888-backend/internal/security"
	"github.com/esc-chula/intania-888-backend/pkg/cache"
	"github.com/esc-chula/intania-888-backend/pkg/config"
	oauthpkg "github.com/esc-chula/intania-888-backend/pkg/oauth"
)

type applicationAuthConfig struct {
	server config.Server
	jwt    config.JWT
	oauth  config.OAuth
}

func (c applicationAuthConfig) GetServer() config.Server { return c.server }
func (c applicationAuthConfig) GetDB() config.DB         { return config.DB{} }
func (c applicationAuthConfig) GetJWT() config.JWT       { return c.jwt }
func (c applicationAuthConfig) GetOAuth() config.OAuth   { return c.oauth }
func (c applicationAuthConfig) GetSession() config.Session {
	return config.Session{
		IdleTTLSeconds:     config.DefaultSessionIdleTTLSeconds,
		AbsoluteTTLSeconds: config.DefaultSessionAbsoluteTTLSeconds,
	}
}
func (c applicationAuthConfig) GetSwagger() config.Swagger         { return config.Swagger{} }
func (c applicationAuthConfig) GetCORS() config.CORS               { return config.CORS{} }
func (c applicationAuthConfig) GetDailyReward() config.DailyReward { return config.DailyReward{} }

type applicationUsers struct {
	mu    sync.Mutex
	users map[string]identity.User
}

func newApplicationUsers() *applicationUsers {
	return &applicationUsers{users: make(map[string]identity.User)}
}

func (r *applicationUsers) Create(_ context.Context, user *identity.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.users[user.ID] = *user
	return nil
}

func (r *applicationUsers) GetByID(_ context.Context, id string) (*identity.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	user, ok := r.users[id]
	if !ok {
		return nil, identity.ErrUserNotFound
	}
	return &user, nil
}

func (r *applicationUsers) GetByEmail(_ context.Context, email string) (*identity.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, user := range r.users {
		if user.Email == email {
			return &user, nil
		}
	}
	return nil, identity.ErrUserNotFound
}

type applicationGoogleClient struct {
	config *oauth2.Config
	info   *oauthpkg.GoogleUserInfo
}

func (c applicationGoogleClient) GetUserInfo(context.Context, string, string) (*oauthpkg.GoogleUserInfo, error) {
	return c.info, nil
}

func (c applicationGoogleClient) OAuthConfig() *oauth2.Config { return c.config }

type applicationPolicyChecker struct{}

func (applicationPolicyChecker) EvaluateLogin(context.Context, string, string, string) (security.PolicyDecision, error) {
	return security.PolicyDecision{Allowed: true}, nil
}

func (applicationPolicyChecker) IsBlacklisted(context.Context, string, string) (bool, error) {
	return false, nil
}

// applicationTokens decodes the public token response without importing HTTP implementation types.
type applicationTokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	UserID       string `json:"user_id"`
}

type applicationTestConfig struct {
	applicationAuthConfig
	redis config.Cache
}

func (c applicationTestConfig) GetCache() config.Cache { return c.redis }

func TestApplicationLoginCodeExchangeRefreshAndRevocation(t *testing.T) {
	address := os.Getenv("INTANIA888_TEST_REDIS_ADDR")
	if address == "" {
		t.Skip("INTANIA888_TEST_REDIS_ADDR is required")
	}
	host, portString, err := net.SplitHostPort(address)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portString)
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("INTANIA_GAMES_CLIENT_SECRET", strings.Repeat("s", 32))
	registry, err := config.LoadAuthRegistry("../../config/auth.development.yaml", "development", "http://localhost:3000", os.Getenv)
	if err != nil {
		t.Fatal(err)
	}
	cfg := applicationTestConfig{
		applicationAuthConfig: applicationAuthConfig{
			server: config.Server{Name: "integration-888", Env: "development"},
			jwt:    config.JWT{AccessTokenSecret: strings.Repeat("j", 32)},
			oauth:  config.OAuth{Registry: registry},
		},
		redis: config.Cache{Host: host, Port: port, Password: os.Getenv("INTANIA888_TEST_REDIS_PASSWORD")},
	}
	client := cache.NewRedisClient(cfg)
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Error(err)
		}
	})
	users := newApplicationUsers()
	google := applicationGoogleClient{
		config: &oauth2.Config{ClientID: "google", RedirectURL: registry.Google.CallbackURI, Endpoint: oauth2.Endpoint{AuthURL: "https://accounts.example.test/authorize"}},
		info:   &oauthpkg.GoogleUserInfo{ID: "integration-player", Email: "player@student.chula.ac.th", Name: "Player", VerifiedEmail: true},
	}
	policy := applicationPolicyChecker{}
	service := auth.NewService(auth.NewRedisRepository(client), users, cfg, google, policy)
	mid := middleware.NewHTTPHandler(middleware.NewService(users, middleware.NewRedisSessionStore(client), cfg, policy), false, cfg.GetSession().IdleTTLSeconds)
	h := auth.NewHTTPHandler(service, mid, cfg, false)
	h.ConfigureApplications(service, client)
	app := fiber.New(fiber.Config{ErrorHandler: apierror.ErrorHandler(nil)})
	router := app.Group("/api/v1")
	h.RegisterRoutes(router, mid.AuthMiddleware)
	h.RegisterExternalRoutes(router.Group("/external"), mid.RequireExternalScope)
	router.Post("/external/deduct-coin", mid.RequireExternalScope(config.ScopeCoinsSpend), func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })

	call := func(method, path, body, cookie, bearer string) *http.Response {
		t.Helper()
		request := httptest.NewRequest(method, path, strings.NewReader(body))
		if body != "" {
			request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			request.SetBasicAuth("intania-games", strings.Repeat("s", 32))
		}
		if cookie != "" {
			request.Header.Set("Cookie", cookie)
		}
		if bearer != "" {
			request.Header.Set("Authorization", "Bearer "+bearer)
		}
		response, err := app.Test(request)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = response.Body.Close() })
		return response
	}

	first := call("GET", "/api/v1/auth/login?client_id=intania-888-web&return_to=%2Fbills", "", "", "")
	second := call("GET", "/api/v1/auth/login?client_id=intania-888-web", "", "", "")
	if first.StatusCode != fiber.StatusSeeOther || second.StatusCode != fiber.StatusSeeOther {
		t.Fatal("login did not redirect")
	}
	firstURL, _ := url.Parse(first.Header.Get("Location"))
	secondURL, _ := url.Parse(second.Header.Get("Location"))
	if firstURL.Query().Get("state") == secondURL.Query().Get("state") || first.Cookies()[0].Name == second.Cookies()[0].Name {
		t.Fatal("parallel login attempts share their binding")
	}

	state := firstURL.Query().Get("state")
	callback := "/api/v1/auth/callback?code=google-code&state=" + state
	wrongBrowser := call("GET", callback, "", "", "")
	if wrongBrowser.StatusCode != fiber.StatusBadRequest {
		t.Fatal("accepted callback without browser binding")
	}
	completed := call("GET", callback, "", first.Cookies()[0].String(), "")
	if completed.StatusCode != fiber.StatusSeeOther || !strings.Contains(completed.Header.Get("Location"), "/register/profile?return_to=%2Fbills") {
		t.Fatalf("new user destination: %d %s", completed.StatusCode, completed.Header.Get("Location"))
	}
	var sessionCookie string
	for _, cookie := range completed.Cookies() {
		if cookie.Name == "session" {
			sessionCookie = cookie.Name + "=" + cookie.Value
		}
	}
	if sessionCookie == "" {
		t.Fatal("session missing")
	}
	if replay := call("GET", callback, "", first.Cookies()[0].String(), ""); replay.StatusCode != fiber.StatusBadRequest {
		t.Fatal("callback replay accepted")
	}

	verifier := strings.Repeat("v", 43)
	digest := sha256.Sum256([]byte(verifier))
	parameters := url.Values{
		"client_id": {"intania-games"}, "redirect_uri": {"http://localhost:8081/auth/callback"}, "response_type": {"code"},
		"state": {"game-state"}, "scope": {"profile.read"}, "code_challenge_method": {"S256"},
		"code_challenge": {base64.RawURLEncoding.EncodeToString(digest[:])},
	}
	authorize := func() string {
		t.Helper()
		response := call("GET", "/api/v1/auth/authorize?"+parameters.Encode(), "", sessionCookie, "")
		location, _ := url.Parse(response.Header.Get("Location"))
		if response.StatusCode != fiber.StatusSeeOther || location.Host != "localhost:8081" || location.Query().Get("state") != "game-state" || location.Query().Get("code") == "" {
			t.Fatalf("authorize: %d %s", response.StatusCode, location)
		}
		return location.Query().Get("code")
	}
	code := authorize()
	exchange := url.Values{"grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {"http://localhost:8081/auth/callback"}, "code_verifier": {strings.Repeat("x", 43)}}
	if response := call("POST", "/api/v1/auth/token", exchange.Encode(), "", ""); response.StatusCode != fiber.StatusBadRequest {
		t.Fatal("wrong verifier accepted")
	}
	exchange.Set("code_verifier", verifier)
	response := call("POST", "/api/v1/auth/token", exchange.Encode(), "", "")
	var tokens applicationTokens
	if err := json.NewDecoder(response.Body).Decode(&tokens); err != nil || response.StatusCode != fiber.StatusOK || tokens.UserID != "integration-player" {
		t.Fatalf("exchange: %d, %v", response.StatusCode, err)
	}
	if replay := call("POST", "/api/v1/auth/token", exchange.Encode(), "", ""); replay.StatusCode != fiber.StatusBadRequest {
		t.Fatal("code replay accepted")
	}
	if me := call("GET", "/api/v1/external/me", "", "", tokens.AccessToken); me.StatusCode != fiber.StatusOK {
		t.Fatalf("profile: %d", me.StatusCode)
	}
	if spend := call("POST", "/api/v1/external/deduct-coin", "", "", tokens.AccessToken); spend.StatusCode != fiber.StatusForbidden {
		t.Fatalf("missing spend scope: %d", spend.StatusCode)
	}

	refresh := url.Values{"grant_type": {"refresh_token"}, "refresh_token": {tokens.RefreshToken}}
	refreshed := call("POST", "/api/v1/auth/token", refresh.Encode(), "", "")
	var renewed applicationTokens
	if err := json.NewDecoder(refreshed.Body).Decode(&renewed); err != nil || refreshed.StatusCode != fiber.StatusOK || renewed.RefreshToken == tokens.RefreshToken {
		t.Fatal("refresh did not rotate")
	}
	if replay := call("POST", "/api/v1/auth/token", refresh.Encode(), "", ""); replay.StatusCode != fiber.StatusBadRequest {
		t.Fatal("refresh replay accepted")
	}
	if me := call("GET", "/api/v1/external/me", "", "", renewed.AccessToken); me.StatusCode != fiber.StatusUnauthorized {
		t.Fatal("refresh replay did not revoke grant")
	}

	// Concurrent exchanges of one code have exactly one winner.
	exchange.Set("code", authorize())
	var statuses [2]int
	var bodies [2][]byte
	var wait sync.WaitGroup
	for i := range statuses {
		wait.Add(1)
		go func(i int) {
			defer wait.Done()
			request := httptest.NewRequest("POST", "/api/v1/auth/token", strings.NewReader(exchange.Encode()))
			request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			request.SetBasicAuth("intania-games", strings.Repeat("s", 32))
			response, err := app.Test(request)
			if err != nil {
				t.Error(err)
				return
			}
			defer response.Body.Close()
			statuses[i] = response.StatusCode
			bodies[i], err = io.ReadAll(response.Body)
			if err != nil {
				t.Error(err)
			}
		}(i)
	}
	wait.Wait()
	winner := -1
	for i, status := range statuses {
		if status == 200 {
			if winner >= 0 {
				t.Fatal("two code exchanges succeeded")
			}
			winner = i
		} else if status != 400 {
			t.Fatalf("unexpected exchange status %d", status)
		}
	}
	if winner < 0 {
		t.Fatal("no code exchange succeeded")
	}
	if err := json.Unmarshal(bodies[winner], &tokens); err != nil {
		t.Fatal(err)
	}
	revoked := call("POST", "/api/v1/auth/revoke", url.Values{"token": {tokens.RefreshToken}}.Encode(), "", "")
	if revoked.StatusCode != fiber.StatusOK {
		t.Fatal("revocation failed")
	}
	if me := call("GET", "/api/v1/external/me", "", "", tokens.AccessToken); me.StatusCode != fiber.StatusUnauthorized {
		t.Fatal("revoked token accepted")
	}
	if browser := call("GET", "/api/v1/auth/me", "", sessionCookie, ""); browser.StatusCode != fiber.StatusOK {
		t.Fatal("delegation revocation revoked browser session")
	}

	if err := service.Logout(context.Background(), strings.TrimPrefix(sessionCookie, "session=")); err != nil {
		t.Fatal(err)
	}
}
