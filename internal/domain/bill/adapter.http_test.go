package bill

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/esc-chula/intania-888-backend/internal/apierror"
	"github.com/esc-chula/intania-888-backend/internal/domain/middleware"
	"github.com/esc-chula/intania-888-backend/internal/identity"
	"github.com/esc-chula/intania-888-backend/internal/value"
	"github.com/esc-chula/intania-888-backend/pkg/config"
)

func newFiberTestApp() *fiber.App {
	return fiber.New(fiber.Config{ErrorHandler: apierror.ErrorHandler(nil)})
}

type contractService struct {
	calls      int
	createErr  error
	voidCalls  int
	voidActor  string
	voidReason string
}

func (s *contractService) CreateBill(_ context.Context, userID string, req *CreateInput) (*Result, error) {
	s.calls++
	if s.createErr != nil {
		return nil, s.createErr
	}

	return &Result{
		ID:     "b",
		UserID: userID,
		Total:  req.Total,
		Status: "PENDING",
		Lines:  []Line{},
	}, nil
}

func TestCreateBillMissingMatchUsesSharedNotFoundContract(t *testing.T) {
	svc := &contractService{createErr: ErrMatchNotFound}
	h := NewHTTPHandler(svc)
	app := newFiberTestApp()
	app.Use(apierror.RequestID())
	app.Post("/bills", func(c *fiber.Ctx) error {
		c.Locals("user", &identity.Profile{ID: "u"})
		return h.CreateBill(c)
	})

	request := httptest.NewRequest("POST", "/bills", strings.NewReader(`{
		"total":"100.00",
		"lines":[{"match_id":"missing","betting_on":"A"}]}`))
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

	if response.StatusCode != fiber.StatusNotFound {
		t.Fatalf("status = %d; want %d", response.StatusCode, fiber.StatusNotFound)
	}

	var body apierror.Response
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Code != "RESOURCE_NOT_FOUND" || body.Message != "Match not found" {
		t.Fatalf("error body = %#v", body)
	}
	if body.RequestID == "" || response.Header.Get(apierror.RequestIDHeader) != body.RequestID {
		t.Fatalf("request ID body/header mismatch: %#v, %q", body, response.Header.Get(apierror.RequestIDHeader))
	}
}

func (*contractService) GetBill(context.Context, string, string) (*Result, error) {
	return nil, nil
}

func (*contractService) GetAllBills(context.Context, string) ([]*Result, error) {
	return nil, nil
}

func (*contractService) GetAllBillsAdmin(context.Context) ([]*Result, error) {
	return nil, nil
}

func (s *contractService) VoidBill(_ context.Context, id, actor, reason string) (*Result, error) {
	s.voidCalls++
	s.voidActor = actor
	s.voidReason = reason
	payout := value.MustMoneyFromMinor(100_00)

	return &Result{
		ID:     id,
		Total:  value.MustMoneyFromMinor(100_00),
		Payout: &payout,
		Status: "VOIDED",
	}, nil
}

func TestCreateBillStrictMoneyContract(t *testing.T) {
	svc := &contractService{}
	h := NewHTTPHandler(svc)
	app := newFiberTestApp()

	app.Post("/bills", func(c *fiber.Ctx) error {
		c.Locals("user", &identity.Profile{ID: "u"})

		return h.CreateBill(c)
	})

	invalidBodies := []string{
		`{"total":100,"lines":[{"match_id":"m","betting_on":"A"}]}`,
		`{"total":"100.00","lines":[{"match_id":"m","betting_on":"A","rate":99}]}`,
	}

	for _, body := range invalidBodies {
		req := httptest.NewRequest("POST", "/bills", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		res, err := app.Test(req)

		if err != nil {
			t.Fatal(err)
		}

		if res.StatusCode != 400 {
			t.Fatalf("body %s returned %d", body, res.StatusCode)
		}
	}

	req := httptest.NewRequest("POST", "/bills", strings.NewReader(`{
		"total":"100.00",
		"lines":[{"match_id":"m","betting_on":"A"}]}`))
	req.Header.Set("Content-Type", "application/json")

	res, err := app.Test(req)

	if err != nil {
		t.Fatal(err)
	}

	if res.StatusCode != 201 {
		t.Fatalf("status=%d", res.StatusCode)
	}

	var got map[string]any

	if err = json.NewDecoder(res.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}

	if got["total"] != "100.00" {
		t.Fatalf("total=%v", got["total"])
	}

	if svc.calls != 1 {
		t.Fatalf("service calls=%d", svc.calls)
	}
}

func TestVoidBillRequiresAdminAndRecordsActor(t *testing.T) {
	svc := &contractService{}
	h := NewHTTPHandler(svc)
	mid := middleware.NewHTTPHandler(
		nil,
		false,
		config.DefaultSessionIdleTTLSeconds,
	)

	app := newFiberTestApp()
	app.Put("/bills/admin/:id/void", func(c *fiber.Ctx) error {
		c.Locals("user", &identity.Profile{ID: "user", RoleID: "USER"})

		return mid.AdminMiddleware(c)
	}, h.VoidBill)

	request := httptest.NewRequest("PUT", "/bills/admin/b/void", strings.NewReader(`{"reason":"operator correction"}`))
	request.Header.Set("Content-Type", "application/json")

	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}

	if response.StatusCode != fiber.StatusForbidden {
		t.Fatalf("non-admin status = %d; want %d", response.StatusCode, fiber.StatusForbidden)
	}

	adminApp := newFiberTestApp()
	adminApp.Put("/bills/admin/:id/void", func(c *fiber.Ctx) error {
		c.Locals("user", &identity.Profile{ID: "admin", RoleID: "ADMIN"})

		return mid.AdminMiddleware(c)
	}, h.VoidBill)

	request = httptest.NewRequest("PUT", "/bills/admin/b/void", strings.NewReader(`{"reason":"operator correction"}`))
	request.Header.Set("Content-Type", "application/json")

	response, err = adminApp.Test(request)
	if err != nil {
		t.Fatal(err)
	}

	if response.StatusCode != fiber.StatusOK {
		t.Fatalf("admin status = %d; want %d", response.StatusCode, fiber.StatusOK)
	}

	if svc.voidCalls != 1 || svc.voidActor != "admin" || svc.voidReason != "operator correction" {
		t.Fatalf("void call = (%d, %q, %q); want (1, admin, operator correction)", svc.voidCalls, svc.voidActor, svc.voidReason)
	}
}
