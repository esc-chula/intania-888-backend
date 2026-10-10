package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/esc-chula/intania-888-backend/internal/apierror"
	"github.com/esc-chula/intania-888-backend/internal/identity"
	"github.com/esc-chula/intania-888-backend/internal/security"
	"github.com/esc-chula/intania-888-backend/pkg/config"
)

type accountFailureService struct {
	ServicePort
	err      error
	contexts []context.Context
}

func (s *accountFailureService) GetSession(ctx context.Context, _ string) (*security.Session, error) {
	s.contexts = append(s.contexts, ctx)

	return &security.Session{
		UserID:    "user",
		CSRFToken: "csrf",
	}, nil
}

func (s *accountFailureService) VerifyExternalGrant(ctx context.Context, _ string, _ string) (string, error) {
	s.contexts = append(s.contexts, ctx)

	return "user", nil
}

func (s *accountFailureService) GetMe(ctx context.Context, _ string) (*identity.Profile, error) {
	s.contexts = append(s.contexts, ctx)

	return nil, s.err
}

func TestAccountFailuresPreserveBrowserAndExternalAuthenticationContract(t *testing.T) {
	tests := []struct {
		name       string
		external   bool
		cause      error
		wantStatus int
		wantCode   string
		clear      bool
	}{
		{"browser missing user", false, identity.ErrUserNotFound, 401, "UNAUTHORIZED", true},
		{"browser timeout", false, context.DeadlineExceeded, 503, "DEPENDENCY_UNAVAILABLE", false},
		{"external missing user", true, identity.ErrUserNotFound, 401, "UNAUTHORIZED", false},
		{"external timeout", true, context.DeadlineExceeded, 503, "DEPENDENCY_UNAVAILABLE", false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			service := &accountFailureService{err: errors.Join(test.cause, errors.New("underlying account lookup"))}
			handler := NewHTTPHandler(service, false, config.DefaultSessionIdleTTLSeconds)
			app := newFiberTestApp()
			app.Use(func(c *fiber.Ctx) error {
				c.SetUserContext(ctx)

				return c.Next()
			})
			authenticate := handler.AuthMiddleware
			if test.external {
				authenticate = handler.RequireExternalScope(config.ScopeProfileRead)
			}
			app.Get("/private", authenticate, func(c *fiber.Ctx) error {
				t.Error("account lookup failure reached the downstream handler")

				return c.SendStatus(fiber.StatusNoContent)
			})

			request := httptest.NewRequest("GET", "/private", nil)
			if test.external {
				request.Header.Set("Authorization", "Bearer external")
			} else {
				request.AddCookie(&http.Cookie{
					Name:  handler.CookieName(),
					Value: "opaque",
				})
			}

			response, err := app.Test(request)
			if err != nil {
				t.Fatal(err)
			}

			defer func() {
				if err := response.Body.Close(); err != nil {
					t.Error(err)
				}
			}()
			var body apierror.Response
			if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if response.StatusCode != test.wantStatus ||
				body.Code != test.wantCode ||
				(len(response.Header.Values("Set-Cookie")) != 0) != test.clear {
				t.Fatalf(
					"account failure response = %d %+v cookies=%v",
					response.StatusCode,
					body,
					response.Header.Values("Set-Cookie"),
				)
			}
			if len(service.contexts) != 2 || service.contexts[0] != ctx || service.contexts[1] != ctx {
				t.Fatalf("middleware lost the request context: %v", service.contexts)
			}
		})
	}
}

func (s *accountFailureService) GetExternalProfile(ctx context.Context, id string) (*identity.Profile, error) {
	return s.GetMe(ctx, id)
}
