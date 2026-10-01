package policy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/esc-chula/intania-888-backend/internal/apierror"
	"github.com/esc-chula/intania-888-backend/internal/identity"
	"github.com/esc-chula/intania-888-backend/internal/security"
)

func newFiberTestApp() *fiber.App {
	return fiber.New(fiber.Config{ErrorHandler: apierror.ErrorHandler(nil)})
}

type fakeHTTPService struct {
	items       []*AccessPolicy
	created     CreateInput
	updatedID   string
	updated     UpdateInput
	disabledID  string
	listResult  ListResult
	createError error
}

func (s *fakeHTTPService) EvaluateLogin(_ context.Context, _, _, _ string) (security.PolicyDecision, error) {
	return security.PolicyDecision{Allowed: true}, nil
}

func (s *fakeHTTPService) IsBlacklisted(_ context.Context, _, _ string) (bool, error) {
	return false, nil
}

func (s *fakeHTTPService) List(_ context.Context, _ ListFilter) (ListResult, error) {
	if s.listResult.Items != nil || s.listResult.HasMore {
		return s.listResult, nil
	}

	return ListResult{Items: s.items}, nil
}

func (s *fakeHTTPService) Create(_ context.Context, input CreateInput) (*AccessPolicy, error) {
	if s.createError != nil {
		return nil, s.createError
	}
	s.created = input

	return &AccessPolicy{
		ID:            "policy-id",
		Kind:          input.Kind,
		PrincipalType: input.PrincipalType,
		Principal:     input.Principal,
		Reason:        input.Reason,
		Enabled:       true,
		ExpiresAt:     input.ExpiresAt,
	}, nil
}

func (s *fakeHTTPService) Update(_ context.Context, id string, input UpdateInput) (*AccessPolicy, error) {
	s.updatedID = id
	s.updated = input

	return &AccessPolicy{
		ID:            id,
		Kind:          KindBlacklist,
		PrincipalType: PrincipalEmail,
		Principal:     "blocked@example.com",
		Enabled:       input.Enabled,
	}, nil
}

func (s *fakeHTTPService) Disable(_ context.Context, id string) (*AccessPolicy, error) {
	s.disabledID = id

	return &AccessPolicy{
		ID:      id,
		Enabled: false,
	}, nil
}

func (*fakeHTTPService) RefreshCache(context.Context) error {
	return nil
}

func TestCreatePolicyRequestAndResponse(t *testing.T) {
	service := &fakeHTTPService{}
	handler := NewHTTPHandler(service)
	app := newFiberTestApp()
	app.Post("/auth/policies", func(c *fiber.Ctx) error {
		c.Locals("user", &identity.Profile{
			ID:     "admin-user",
			RoleID: security.RoleAdmin,
		})

		return handler.Create(c)
	})

	request := httptest.NewRequest(
		http.MethodPost,
		"/auth/policies",
		strings.NewReader(`{"kind":"blacklist","principal_type":"email","principal":" BLOCKED@EXAMPLE.COM ","reason":"manual block","expires_at":null}`),
	)
	request.Header.Set("Content-Type", "application/json")

	response, err := app.Test(request)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusCreated)
	}
	if service.created.Kind != KindBlacklist ||
		service.created.PrincipalType != PrincipalEmail ||
		service.created.Principal != " BLOCKED@EXAMPLE.COM " {
		t.Fatalf("created input = %+v", service.created)
	}
}

func TestUpdatePolicyParsesNullableExpiryAndEnabled(t *testing.T) {
	service := &fakeHTTPService{}
	handler := NewHTTPHandler(service)
	app := newFiberTestApp()
	app.Patch("/auth/policies/:id", func(c *fiber.Ctx) error {
		c.Locals("user", &identity.Profile{
			ID:     "admin-user",
			RoleID: security.RoleAdmin,
		})

		return handler.Update(c)
	})

	request := httptest.NewRequest(
		http.MethodPatch,
		"/auth/policies/policy-id",
		strings.NewReader(`{"reason":"updated","expires_at":null,"enabled":false}`),
	)
	request.Header.Set("Content-Type", "application/json")

	response, err := app.Test(request)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusOK)
	}
	if service.updatedID != "policy-id" ||
		!service.updated.ReasonSet ||
		service.updated.Reason != "updated" ||
		!service.updated.ExpiresAtSet ||
		service.updated.ExpiresAt != nil ||
		!service.updated.EnabledSet ||
		service.updated.Enabled {
		t.Fatalf("update input = %+v", service.updated)
	}
}

func TestListPolicyReturnsCursor(t *testing.T) {
	service := &fakeHTTPService{
		listResult: ListResult{
			Items: []*AccessPolicy{{
				ID:            "policy-id",
				Kind:          KindAllowlist,
				PrincipalType: PrincipalEmail,
				Principal:     "user@example.com",
				Enabled:       true,
			}},
			HasMore: true,
		},
	}
	handler := NewHTTPHandler(service)
	app := newFiberTestApp()
	app.Get("/auth/policies", handler.List)

	response, err := app.Test(httptest.NewRequest(http.MethodGet, "/auth/policies?limit=1", nil))
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusOK)
	}
	if !strings.Contains(response.Header.Get("Content-Type"), "application/json") {
		t.Fatalf("content type = %q", response.Header.Get("Content-Type"))
	}
}

func TestPolicyErrorsMapToContractStatuses(t *testing.T) {
	service := &fakeHTTPService{createError: ErrPolicyConflict}
	handler := NewHTTPHandler(service)
	app := newFiberTestApp()
	app.Post("/auth/policies", handler.Create)

	request := httptest.NewRequest(
		http.MethodPost,
		"/auth/policies",
		strings.NewReader(`{"kind":"allowlist","principal_type":"email","principal":"student@example.test","reason":"test"}`),
	)
	request.Header.Set("Content-Type", "application/json")

	response, err := app.Test(request)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	if response.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusConflict)
	}
}

func TestCursorHelpers(t *testing.T) {
	cursor := encodeCursor(25)
	offset, err := decodeCursor(cursor)
	if err != nil || offset != 25 {
		t.Fatalf("cursor = %q, offset = %d, error = %v", cursor, offset, err)
	}
	if _, err := decodeCursor("not-a-cursor"); err == nil {
		t.Fatal("invalid cursor decoded successfully")
	}
	if _, err := parseLimit("0"); err == nil {
		t.Fatal("invalid limit accepted")
	}
}
