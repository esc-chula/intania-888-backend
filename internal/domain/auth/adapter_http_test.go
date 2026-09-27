package auth

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/esc-chula/intania-888-backend/internal/domain/middleware"
	"github.com/esc-chula/intania-888-backend/internal/model"
	"github.com/esc-chula/intania-888-backend/pkg/config"
	"github.com/esc-chula/intania-888-backend/utils"
	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

type fakeAuthService struct {
	login       *OAuthLogin
	credentials *SessionCredentials
	redirect    string
	logoutErr   error
}

func (s fakeAuthService) StartOAuthLogin() (*OAuthLogin, error) { return s.login, nil }

func (s fakeAuthService) VerifyOAuthLogin(string, string, string, string) (*SessionCredentials, error) {
	return s.credentials, nil
}

func (s fakeAuthService) Logout(string) error { return s.logoutErr }

func (s fakeAuthService) GetPostLoginRedirectURL() string { return s.redirect }

func (s fakeAuthService) IssueExternalToken(string) (string, string, error) {
	return "token", "id", nil
}

func (s fakeAuthService) RevokeExternalToken(string) error { return nil }

func newHTTPConfig(env string) config.Config {
	return authTestConfig{server: config.Server{Env: env}, oauth: config.OAuth{StateExpiration: 600, PostLoginRedirectUrl: "https://frontend.example.test/app?source=oauth"}}
}

func TestLoginAndCallbackCookiePolicy(t *testing.T) {
	for _, tc := range []struct {
		env, name, oauth string
		production       bool
	}{{"development", "session", "oauth", false}, {"production", "__Host-session", "__Host-oauth", true}} {
		t.Run(tc.env, func(t *testing.T) {
			svc := fakeAuthService{login: &OAuthLogin{URL: "https://accounts.example.test/?state=abc", State: "abc"}, credentials: &SessionCredentials{SessionID: "opaque", IsNewUser: true}, redirect: "https://frontend.example.test/app"}
			h := NewAuthHttpHandler(svc, newHTTPConfig(tc.env), tc.production)

			app := fiber.New()
			app.Get("/login", h.Login)
			app.Get("/callback", h.OAuthCallback)

			response, _ := app.Test(httptest.NewRequest(http.MethodGet, "/login", nil))
			if response.StatusCode != 200 {
				t.Fatal(response.StatusCode)
			}
			if !strings.Contains(strings.Join(response.Header.Values("Set-Cookie"), " "), tc.oauth+"=abc") {
				t.Fatal("oauth cookie missing")
			}

			req := httptest.NewRequest(http.MethodGet, "/callback?code=code&state=abc", nil)
			req.Header.Set("Cookie", tc.oauth+"=abc")
			response, _ = app.Test(req)
			if response.StatusCode != 302 {
				t.Fatal(response.StatusCode)
			}

			cookies := strings.Join(response.Header.Values("Set-Cookie"), "\n")
			if !strings.Contains(cookies, tc.name+"=opaque") || !strings.Contains(cookies, "HttpOnly") || !strings.Contains(cookies, "SameSite=Lax") || strings.Contains(cookies, "Domain=") {
				t.Fatalf("bad cookies: %s", cookies)
			}
			if tc.production && !strings.Contains(cookies, "secure") {
				t.Fatalf("missing Secure: %s", cookies)
			}
			if !tc.production && strings.Contains(cookies, "secure") {
				t.Fatalf("local cookie requires HTTPS: %s", cookies)
			}
			if !strings.Contains(response.Header.Get("Location"), "is_new_user=true") {
				t.Fatal("missing new user marker")
			}
		})
	}
}

func TestMeReturnsCSRFOnlyToBrowser(t *testing.T) {
	h := NewAuthHttpHandler(fakeAuthService{}, newHTTPConfig("development"), false)

	app := fiber.New()
	app.Get("/me", func(c *fiber.Ctx) error {
		c.Locals("user", &model.UserDto{Id: "u"})
		c.Locals("csrf_token", "secret")
		return c.Next()
	}, h.GetMe)

	response, _ := app.Test(httptest.NewRequest(http.MethodGet, "/me", nil))

	body := make([]byte, response.ContentLength)
	_, _ = response.Body.Read(body)
	if !strings.Contains(string(body), `"csrf_token":"secret"`) || strings.Contains(string(body), "access_token") {
		t.Fatalf("body: %s", body)
	}
	if utils.SessionCookieName(false) != "session" {
		t.Fatal("wrong local cookie")
	}
}

type logoutMiddlewareService struct {
	session *model.SessionRecord
	err     error
}

func (s logoutMiddlewareService) GetSession(string) (*model.SessionRecord, error) {
	return s.session, s.err
}

func (s logoutMiddlewareService) VerifyExternalToken(string) (string, error) { return "", nil }

func (s logoutMiddlewareService) GetMe(string) (*model.UserDto, error) { return nil, nil }

func (s logoutMiddlewareService) IsBlacklisted(string, string) (bool, error) { return false, nil }

func TestLogoutIdempotenceAndRetry(t *testing.T) {
	for _, tc := range []struct {
		name                string
		session             *model.SessionRecord
		storeErr, logoutErr error
		want                int
		cleared             bool
	}{
		{"absent", nil, nil, nil, 204, true},
		{"expired", nil, middleware.ErrSessionMissing, nil, 204, true},
		{"redis unavailable", nil, errors.New("redis down"), nil, 503, false},
		{"revoke unavailable", &model.SessionRecord{CSRFToken: "csrf"}, nil, errors.New("redis down"), 503, false},
		{"revoked", &model.SessionRecord{CSRFToken: "csrf"}, nil, nil, 204, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := NewAuthHttpHandler(fakeAuthService{logoutErr: tc.logoutErr}, newHTTPConfig("development"), false)
			h.mid = middleware.NewMiddlewareHttpHandler(
				logoutMiddlewareService{session: tc.session, err: tc.storeErr},
				zap.NewNop(),
				false,
				config.DefaultSessionIdleTTLSeconds,
			)

			app := fiber.New()
			app.Post("/logout", h.Logout)

			req := httptest.NewRequest(http.MethodPost, "/logout", nil)
			if tc.name != "absent" {
				req.Header.Set("Cookie", "session=opaque")
			}
			req.Header.Set("X-CSRF-Token", "csrf")

			response, _ := app.Test(req)
			if response.StatusCode != tc.want {
				t.Fatalf("status %d", response.StatusCode)
			}

			cleared := len(response.Header.Values("Set-Cookie")) > 0
			if cleared != tc.cleared {
				t.Fatalf("cookie cleared=%v", cleared)
			}
		})
	}
}
