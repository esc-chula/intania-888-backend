package user

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"

	"github.com/esc-chula/intania-888-backend/internal/apierror"
	"github.com/esc-chula/intania-888-backend/internal/httpidentity"
	"github.com/esc-chula/intania-888-backend/internal/identity"
	"github.com/esc-chula/intania-888-backend/internal/value"
)

type fakeHTTPService struct {
	ServicePort
	err     error
	actorID string
	input   ProfilePatch
}

func (s *fakeHTTPService) GetUser(context.Context, string) (*identity.Profile, error) {
	return nil, s.err
}

func (s *fakeHTTPService) UpdateOwnProfile(_ context.Context, actorID string, input ProfilePatch) (*identity.Profile, error) {
	s.actorID = actorID
	s.input = input
	name := ""
	if input.Name != nil {
		name = *input.Name
	}
	return &identity.Profile{ID: actorID, Email: "actor@example.test", Name: name, RoleID: "USER", RemainingCoin: value.MustMoneyFromMinor(12345)}, nil
}

func TestUserHTTPErrorMappingPreservesStatusAndRedactsStorageErrors(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{"not found", ErrUserNotFound, 404, "RESOURCE_NOT_FOUND"},
		{"storage failure", errors.New("database secret"), 500, "INTERNAL_ERROR"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app := fiber.New(fiber.Config{ErrorHandler: apierror.ErrorHandler(zap.NewNop())})
			app.Use(apierror.RequestID())
			app.Get("/users/:id", NewHTTPHandler(&fakeHTTPService{err: test.err}).GetUser)
			response, err := app.Test(httptest.NewRequest("GET", "/users/user", nil))
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
			if response.StatusCode != test.wantStatus || body.Code != test.wantCode {
				t.Fatalf("status/body = %d %+v", response.StatusCode, body)
			}
			if strings.Contains(body.Message, "database secret") || body.RequestID == "" {
				t.Fatalf("unsafe error response: %+v", body)
			}
		})
	}
}

func TestDeprecatedUpdateUserDelegatesToOwnProfileAndPreservesWireShape(t *testing.T) {
	service := &fakeHTTPService{}
	handler := NewHTTPHandler(service)
	app := fiber.New(fiber.Config{ErrorHandler: apierror.ErrorHandler(zap.NewNop())})
	app.Patch("/users/:id", func(c *fiber.Ctx) error {
		httpidentity.SetProfile(c, &identity.Profile{ID: "actor", Email: "actor@example.test", RoleID: "USER"})
		return handler.UpdateUser(c)
	})
	request := httptest.NewRequest("PATCH", "/users/actor", strings.NewReader(`{"name":"Updated"}`))
	request.Header.Set("Content-Type", "application/json")
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := response.Body.Close(); err != nil {
			t.Error(err)
		}
	}()
	var body map[string]any
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if service.actorID != "actor" || service.input.Name == nil || *service.input.Name != "Updated" {
		t.Fatalf("profile update was not delegated for the authenticated actor: actor=%q input=%+v", service.actorID, service.input)
	}
	if response.StatusCode != fiber.StatusOK || body["remaining_coin"] != "123.45" || body["nick_name"] != nil || body["group_id"] != nil || body["created_at"] != "0001-01-01T00:00:00Z" || len(body) != 8 {
		t.Fatalf("wire shape changed: status=%d body=%+v", response.StatusCode, body)
	}
}
