package bill

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/esc-chula/intania-888-backend/internal/domain/middleware"
	"github.com/esc-chula/intania-888-backend/internal/model"
	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

type contractService struct {
	calls      int
	voidCalls  int
	voidActor  string
	voidReason string
}

func (s *contractService) CreateBill(userID string, req *model.CreateBillRequest) (*model.BillHeadDto, error) {
	s.calls++

	return &model.BillHeadDto{
		Id:     "b",
		UserId: userID,
		Total:  req.Total,
		Status: "PENDING",
		Lines:  []*model.BillLineDto{},
	}, nil
}

func (*contractService) GetBill(string, string) (*model.BillHeadDto, error) {
	return nil, nil
}

func (*contractService) GetAllBills(string) ([]*model.BillHeadDto, error) {
	return nil, nil
}

func (*contractService) GetAllBillsAdmin() ([]*model.BillHeadDto, error) {
	return nil, nil
}

func (s *contractService) VoidBill(id, actor, reason string) (*model.BillHeadDto, error) {
	s.voidCalls++
	s.voidActor = actor
	s.voidReason = reason
	payout := model.MustMoneyFromMinor(10000)

	return &model.BillHeadDto{
		Id:     id,
		Total:  model.MustMoneyFromMinor(10000),
		Payout: &payout,
		Status: "VOIDED",
	}, nil
}

func TestCreateBillStrictMoneyContract(t *testing.T) {
	svc := &contractService{}
	h := NewBillHttpHandler(svc)
	app := fiber.New()

	app.Post("/bills", func(c *fiber.Ctx) error {
		c.Locals("user", &model.UserDto{Id: "u"})

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

	req := httptest.NewRequest("POST", "/bills", strings.NewReader(`{"total":"100.00","lines":[{"match_id":"m","betting_on":"A"}]}`))
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
	h := NewBillHttpHandler(svc)
	mid := middleware.NewMiddlewareHttpHandler(nil, zap.NewNop())

	app := fiber.New()
	app.Put("/bills/admin/:id/void", func(c *fiber.Ctx) error {
		c.Locals("user", &model.UserDto{Id: "user", RoleId: "USER"})

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

	adminApp := fiber.New()
	adminApp.Put("/bills/admin/:id/void", func(c *fiber.Ctx) error {
		c.Locals("user", &model.UserDto{Id: "admin", RoleId: "ADMIN"})

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
