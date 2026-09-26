package auth

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/esc-chula/intania-888-backend/internal/model"
	"github.com/esc-chula/intania-888-backend/pkg/config"
	"github.com/esc-chula/intania-888-backend/utils"
	"github.com/gofiber/fiber/v2"
)

type fakeAuthService struct {
	login       *OAuthLogin
	credentials *SessionCredentials
	refresh     *SessionCredentials
	redirect    string
	logoutID    string
}

func (s *fakeAuthService) StartOAuthLogin() (*OAuthLogin, error) { return s.login, nil }

func (s *fakeAuthService) VerifyOAuthLogin(string, string, string) (*SessionCredentials, error) {
	return s.credentials, nil
}

func (s *fakeAuthService) RefreshToken(string) (*SessionCredentials, error) {
	return s.refresh, nil
}

func (s *fakeAuthService) Logout(sessionID string) error {
	s.logoutID = sessionID
	return nil
}

func (s *fakeAuthService) GetPostLoginRedirectURL() string { return s.redirect }

func newAuthHandlerTestConfig() config.Config {
	return authTestConfig{
		server: config.Server{Env: "development"},
		jwt: config.Jwt{
			RefreshTokenExpiration: 3600,
		},
		oauth: config.OAuth{
			StateExpiration:      600,
			PostLoginRedirectUrl: "https://frontend.example.test/app?source=oauth",
		},
	}
}

func TestLoginRejectsCallerRedirectAndSetsHttpOnlyStateCookie(t *testing.T) {
	service := &fakeAuthService{
		login: &OAuthLogin{
			URL:   "https://accounts.example.test/authorize?state=state-value",
			State: "state-value",
		},
	}
	handler := NewAuthHttpHandler(service, newAuthHandlerTestConfig())
	app := fiber.New()
	app.Get("/login", handler.Login)

	response, err := app.Test(httptest.NewRequest(http.MethodGet, "/login?redirect_to=https://evil.example", nil))
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusBadRequest)
	}

	response, err = app.Test(httptest.NewRequest(http.MethodGet, "/login", nil))
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusOK)
	}
	setCookies := strings.Join(response.Header.Values("Set-Cookie"), "\n")
	if !strings.Contains(setCookies, "oauth_state=state-value") || !strings.Contains(setCookies, "HttpOnly") || !strings.Contains(setCookies, "SameSite=Lax") {
		t.Fatalf("state cookie missing security attributes: %q", setCookies)
	}
}

func TestOAuthCallbackSetsCookiesAndOnlyFixedRedirectMetadata(t *testing.T) {
	service := &fakeAuthService{
		credentials: &SessionCredentials{
			AccessToken:  "access-secret-value",
			RefreshToken: "refresh-secret-value",
			ExpiresIn:    300,
			IsNewUser:    true,
		},
		redirect: "https://frontend.example.test/app?source=oauth",
	}
	handler := NewAuthHttpHandler(service, newAuthHandlerTestConfig())
	app := fiber.New()
	app.Get("/callback", handler.OAuthCallback)

	request := httptest.NewRequest(http.MethodGet, "/callback?code=google-code&state=state-value", nil)
	request.Header.Set("Cookie", "oauth_state=state-value")
	response, err := app.Test(request)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	if response.StatusCode != http.StatusFound {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusFound)
	}
	if response.Header.Get("Cache-Control") != "no-store" || response.Header.Get("Referrer-Policy") != "no-referrer" {
		t.Fatalf("callback response is cacheable or referrable: cache-control=%q referrer-policy=%q", response.Header.Get("Cache-Control"), response.Header.Get("Referrer-Policy"))
	}
	location := response.Header.Get("Location")
	if !strings.Contains(location, "is_new_user=true") || strings.Contains(location, "access-secret-value") || strings.Contains(location, "refresh-secret-value") {
		t.Fatalf("unsafe callback redirect = %q", location)
	}
	setCookies := strings.Join(response.Header.Values("Set-Cookie"), "\n")
	if !strings.Contains(setCookies, "access_token=access-secret-value") || !strings.Contains(setCookies, "refresh_token=refresh-secret-value") || !strings.Contains(setCookies, "csrf_token=") {
		t.Fatalf("session cookies missing: %q", setCookies)
	}
	if strings.Contains(setCookies, "Domain=") {
		t.Fatalf("session cookies unexpectedly set a Domain: %q", setCookies)
	}
	if !strings.Contains(setCookies, "access_token=access-secret-value; max-age=300; path=/; HttpOnly") {
		t.Fatalf("access cookie is not HttpOnly: %q", setCookies)
	}
}

func TestProductionSessionCookiesAreSecureAndHostOnly(t *testing.T) {
	service := &fakeAuthService{
		credentials: &SessionCredentials{
			AccessToken:  "access-value",
			RefreshToken: "refresh-value",
			ExpiresIn:    300,
		},
		redirect: "https://frontend.example.test/app",
	}
	config := authTestConfig{
		server: config.Server{Env: "production"},
		jwt: config.Jwt{
			RefreshTokenExpiration: 3600,
		},
		oauth: config.OAuth{
			StateExpiration:      600,
			CookieSameSite:       "lax",
			CookieSecure:         true,
			PostLoginRedirectUrl: "https://frontend.example.test/app",
		},
	}
	handler := NewAuthHttpHandler(service, config)
	app := fiber.New()
	app.Get("/callback", handler.OAuthCallback)

	request := httptest.NewRequest(http.MethodGet, "/callback?code=google-code&state=state-value", nil)
	request.Header.Set("Cookie", "oauth_state=state-value")
	response, err := app.Test(request)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	for _, cookie := range response.Cookies() {
		switch cookie.Name {
		case utils.AccessTokenCookieName, utils.RefreshTokenCookieName, utils.CSRFTokenCookieName:
			if !cookie.Secure {
				t.Fatalf("cookie %q is not Secure", cookie.Name)
			}
			if cookie.Domain != "" {
				t.Fatalf("cookie %q has a Domain: %q", cookie.Name, cookie.Domain)
			}
		case utils.OAuthStateCookieName:
			if !cookie.Secure {
				t.Fatalf("OAuth state cookie is not Secure")
			}
		}
	}
}

func TestExistingUserRedirectOmitsNewUserFlagAndRefreshHasNoJSONCredentials(t *testing.T) {
	service := &fakeAuthService{
		credentials: &SessionCredentials{
			AccessToken:  "access-value",
			RefreshToken: "refresh-value",
			ExpiresIn:    300,
		},
		refresh: &SessionCredentials{
			AccessToken:  "new-access-value",
			RefreshToken: "new-refresh-value",
			ExpiresIn:    300,
		},
		redirect: "https://frontend.example.test/app?is_new_user=false",
	}
	handler := NewAuthHttpHandler(service, newAuthHandlerTestConfig())
	app := fiber.New()
	app.Get("/callback", handler.OAuthCallback)
	app.Post("/refresh", handler.RefreshToken)

	request := httptest.NewRequest(http.MethodGet, "/callback?code=google-code&state=state-value", nil)
	request.Header.Set("Cookie", "oauth_state=state-value")
	response, err := app.Test(request)
	if err != nil {
		t.Fatalf("callback app.Test() error = %v", err)
	}
	if strings.Contains(response.Header.Get("Location"), "is_new_user") {
		t.Fatalf("existing-user redirect has new-user marker: %q", response.Header.Get("Location"))
	}

	request = httptest.NewRequest(http.MethodPost, "/refresh", nil)
	request.Header.Set("Cookie", utils.RefreshTokenCookieName+"=refresh-value")
	response, err = app.Test(request)
	if err != nil {
		t.Fatalf("refresh app.Test() error = %v", err)
	}
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("refresh status = %d, want %d", response.StatusCode, http.StatusNoContent)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read refresh body: %v", err)
	}
	if len(body) != 0 || strings.Contains(string(body), "access_token") || strings.Contains(string(body), "refresh_token") {
		t.Fatalf("refresh response contains credentials: %q", body)
	}
}

func TestGetMeAndExternalProfileDoNotExposeCredentials(t *testing.T) {
	handler := NewAuthHttpHandler(&fakeAuthService{})
	app := fiber.New()
	app.Get("/me", func(c *fiber.Ctx) error {
		c.Locals("user", &model.UserDto{Id: "user", Email: "user@example.test"})
		return handler.GetMe(c)
	})
	response, err := app.Test(httptest.NewRequest(http.MethodGet, "/me", nil))
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusOK)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if strings.Contains(string(body), "access_token") || strings.Contains(string(body), "refresh_token") {
		t.Fatalf("profile response contains credentials: %q", body)
	}
}
