package auth

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/esc-chula/intania-888-backend/internal/apierror"
	"github.com/esc-chula/intania-888-backend/internal/domain/middleware"
	"github.com/esc-chula/intania-888-backend/internal/identity"
	"github.com/esc-chula/intania-888-backend/internal/security"
	"github.com/esc-chula/intania-888-backend/pkg/config"
)

func newFiberTestApp() *fiber.App {
	return fiber.New(fiber.Config{ErrorHandler: apierror.ErrorHandler(nil)})
}

type fakeAuthService struct {
	login       *OAuthLogin
	credentials *SessionCredentials
	logoutErr   error
}

func (s fakeAuthService) StartOAuthLogin(context.Context) (*OAuthLogin, error) { return s.login, nil }

func (s fakeAuthService) VerifyOAuthLogin(context.Context, string, string, string, string) (*SessionCredentials, error) {
	return s.credentials, nil
}

func (s fakeAuthService) Logout(context.Context, string) error { return s.logoutErr }

func newHTTPConfig(env string) config.Config {
	return authTestConfig{server: config.Server{Env: env}, oauth: config.OAuth{Registry: &config.AuthRegistry{Lifetimes: config.AuthLifetimes{Login: 600}}}}
}

func TestLoginAndCallbackCookiePolicy(t *testing.T) {
	for _, tc := range []struct {
		env, name  string
		production bool
	}{{"development", "session", false}, {"production", "__Host-session", true}} {
		t.Run(tc.env, func(t *testing.T) {
			h := &ApplicationHTTPHandler{production: tc.production}
			app := newFiberTestApp()
			app.Get("/callback", func(c *fiber.Ctx) error {
				h.cookie(c, security.SessionCookieName(tc.production), "opaque", 600)
				h.clear(c, h.cookiePrefix()+"transaction")
				return c.Redirect("https://frontend.example.test/register/profile", fiber.StatusSeeOther)
			})

			response, err := app.Test(httptest.NewRequest(http.MethodGet, "/callback", nil))
			if err != nil {
				t.Fatal(err)
			}
			if err := response.Body.Close(); err != nil {
				t.Fatal(err)
			}
			if response.StatusCode != fiber.StatusSeeOther {
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
		})
	}
}

func TestMapAuthError(t *testing.T) {
	unknownError := errors.New("provider unavailable")

	tests := []struct {
		name      string
		err       error
		status    int
		code      string
		message   string
		wantCause error
	}{
		{
			name:      "wrapped invalid state",
			err:       errors.Join(errors.New("oauth provider"), ErrInvalidOAuthState),
			status:    fiber.StatusBadRequest,
			code:      "INVALID_REQUEST",
			message:   "Invalid or expired OAuth state",
			wantCause: ErrInvalidOAuthState,
		},
		{
			name:      "unverified email",
			err:       ErrUnverifiedEmail,
			status:    fiber.StatusForbidden,
			code:      "FORBIDDEN",
			message:   "Email is not allowed",
			wantCause: ErrUnverifiedEmail,
		},
		{
			name:      "disallowed email",
			err:       ErrEmailNotAllowed,
			status:    fiber.StatusForbidden,
			code:      "FORBIDDEN",
			message:   "Email is not allowed",
			wantCause: ErrEmailNotAllowed,
		},
		{
			name:      "policy unavailable",
			err:       security.ErrPolicyUnavailable,
			status:    fiber.StatusServiceUnavailable,
			code:      "DEPENDENCY_UNAVAILABLE",
			message:   "Access policy is unavailable",
			wantCause: security.ErrPolicyUnavailable,
		},
		{
			name:      "unknown oauth failure",
			err:       unknownError,
			status:    fiber.StatusServiceUnavailable,
			code:      "DEPENDENCY_UNAVAILABLE",
			message:   "OAuth login is unavailable",
			wantCause: unknownError,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mapped := mapAuthError(test.err)

			var got *apierror.Error
			if !errors.As(mapped, &got) {
				t.Fatalf("mapped error type = %T; want *apierror.Error", mapped)
			}
			if got.Status != test.status || got.Code != test.code || got.Message != test.message {
				t.Fatalf("mapped error = %#v", got)
			}
			if !errors.Is(mapped, test.wantCause) {
				t.Fatalf("mapped error %v does not preserve cause %v", mapped, test.wantCause)
			}
		})
	}
}

func TestMeReturnsCSRFOnlyToBrowser(t *testing.T) {
	h := NewHTTPHandler(fakeAuthService{}, nil, newHTTPConfig("development"), false)

	app := newFiberTestApp()
	app.Get("/me", func(c *fiber.Ctx) error {
		c.Locals("user", &identity.Profile{ID: "u"})
		c.Locals("csrf_token", "secret")
		return c.Next()
	}, h.GetMe)

	response, err := app.Test(httptest.NewRequest(http.MethodGet, "/me", nil))
	if err != nil {
		t.Fatal(err)
	}
	if err != nil {
		t.Fatal(err)
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if err := response.Body.Close(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), `"csrf_token":"secret"`) || strings.Contains(string(body), "access_token") {
		t.Fatalf("body: %s", body)
	}
	if security.SessionCookieName(false) != "session" {
		t.Fatal("wrong local cookie")
	}
}

type logoutMiddlewareService struct {
	session *security.Session
	err     error
}

func (s logoutMiddlewareService) GetSession(context.Context, string) (*security.Session, error) {
	return s.session, s.err
}

func (s logoutMiddlewareService) VerifyScopedExternalToken(context.Context, string, string) (string, error) {
	return "", nil
}

func (s logoutMiddlewareService) GetMe(context.Context, string) (*identity.Profile, error) {
	return nil, nil
}

func (s logoutMiddlewareService) IsBlacklisted(context.Context, string, string) (bool, error) {
	return false, nil
}

func TestLogoutIdempotenceAndRetry(t *testing.T) {
	for _, tc := range []struct {
		name                string
		session             *security.Session
		storeErr, logoutErr error
		want                int
		cleared             bool
	}{
		{"absent", nil, nil, nil, 204, true},
		{"expired", nil, middleware.ErrSessionMissing, nil, 204, true},
		{"redis unavailable", nil, errors.New("redis down"), nil, 503, false},
		{"revoke unavailable", &security.Session{CSRFToken: "csrf"}, nil, errors.New("redis down"), 503, false},
		{"revoked", &security.Session{CSRFToken: "csrf"}, nil, nil, 204, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := NewHTTPHandler(fakeAuthService{logoutErr: tc.logoutErr}, nil, newHTTPConfig("development"), false)
			h.sessions = middleware.NewHTTPHandler(
				logoutMiddlewareService{session: tc.session, err: tc.storeErr},

				false,
				config.DefaultSessionIdleTTLSeconds,
			)

			app := newFiberTestApp()
			app.Post("/logout", h.Logout)

			req := httptest.NewRequest(http.MethodPost, "/logout", nil)
			if tc.name != "absent" {
				req.Header.Set("Cookie", "session=opaque")
			}
			req.Header.Set("X-CSRF-Token", "csrf")

			response, err := app.Test(req)
			if err != nil {
				t.Fatal(err)
			}
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
