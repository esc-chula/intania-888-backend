package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/esc-chula/intania-888-backend/internal/httplimit"
	"github.com/esc-chula/intania-888-backend/internal/identity"
	"github.com/esc-chula/intania-888-backend/internal/security"
	"github.com/esc-chula/intania-888-backend/pkg/config"
)

type limitedAccountService struct {
	accountCalls int
	policyCalls  int
}

func (s *limitedAccountService) GetSession(_ context.Context, id string) (*security.Session, error) {
	user := strings.Split(id, ":")[0]
	return &security.Session{UserID: user, CSRFToken: "csrf"}, nil
}

func (s *limitedAccountService) VerifyExternalToken(token string) (*security.DelegatedClaims, error) {
	if token == "invalid" {
		return nil, ErrExternalMissing
	}

	return &security.DelegatedClaims{ClientID: token, DelegationID: token}, nil
}

func (s *limitedAccountService) VerifyExternalGrant(_ context.Context, claims *security.DelegatedClaims, _ string) (string, error) {
	if claims.DelegationID == "revoked" {
		return "", ErrExternalMissing
	}

	return "user", nil
}

func (s *limitedAccountService) GetMe(_ context.Context, id string) (*identity.Profile, error) {
	s.accountCalls++
	return &identity.Profile{ID: id, Email: "user@example.test"}, nil
}

func (s *limitedAccountService) GetExternalProfile(ctx context.Context, id string) (*identity.Profile, error) {
	return s.GetMe(ctx, id)
}

func (s *limitedAccountService) IsBlacklisted(context.Context, string, string) (bool, error) {
	s.policyCalls++
	return false, nil
}

func limitedResponse(t *testing.T, app *fiber.App, request *http.Request, want int) {
	t.Helper()
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	if err := response.Body.Close(); err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != want {
		t.Fatalf("status = %d, want %d", response.StatusCode, want)
	}
	if want == 429 && response.Header.Get("Retry-After") == "" {
		t.Fatal("missing Retry-After")
	}
}

func TestBrowserQuotaSharesSessionsButSeparatesAccountsBeforeQueries(t *testing.T) {
	service := &limitedAccountService{}
	handler := NewHTTPHandler(service, false, config.DefaultSessionIdleTTLSeconds)
	handler.ConfigureRateLimits(httplimit.New("browser", config.RatePolicy{PerMinute: 1, Burst: 1}, nil).Check, nil, nil)
	app := newFiberTestApp()
	reached := 0
	app.Get("/private", handler.AuthMiddleware, func(c *fiber.Ctx) error { reached++; return c.SendStatus(204) })
	for _, tc := range []struct {
		session string
		want    int
	}{{"A:first", 204}, {"A:second", 429}, {"B:first", 204}} {
		request := httptest.NewRequest("GET", "/private", nil)
		request.AddCookie(&http.Cookie{Name: handler.CookieName(), Value: tc.session})
		limitedResponse(t, app, request, tc.want)
	}
	if service.accountCalls != 2 || service.policyCalls != 2 || reached != 2 {
		t.Fatalf("calls: accounts=%d policies=%d reached=%d", service.accountCalls, service.policyCalls, reached)
	}
}

func TestRejectedCredentialsDoNotConsumeUserQuotaAndQuotaStopsQueries(t *testing.T) {
	service := &limitedAccountService{}
	handler := NewHTTPHandler(service, false, config.DefaultSessionIdleTTLSeconds)
	client := httplimit.New("client", config.RatePolicy{PerMinute: 1, Burst: 1}, nil)
	invalid := httplimit.New("invalid", config.RatePolicy{PerMinute: 1, Burst: 1}, nil)
	handler.ConfigureRateLimits(nil, client.Check, invalid.Check)
	limit := httplimit.New("profile", config.RatePolicy{PerMinute: 1, Burst: 1}, nil)
	app := newFiberTestApp()
	app.Get("/profile", handler.RequireExternalScope(config.ScopeProfileRead, limit.Check), func(c *fiber.Ctx) error {
		return c.SendStatus(204)
	})
	for _, tc := range []struct {
		token string
		want  int
	}{{"invalid", 401}, {"invalid", 429}, {"revoked", 429}, {"clientA", 204}, {"clientA", 429}, {"clientB", 429}} {
		request := httptest.NewRequest("GET", "/profile", nil)
		request.Header.Set("Authorization", "Bearer "+tc.token)
		limitedResponse(t, app, request, tc.want)
	}
	if service.accountCalls != 1 || service.policyCalls != 1 {
		t.Fatalf("rejected grant or quota reached account checks: %+v", service)
	}
}
