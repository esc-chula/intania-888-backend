package bill

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/esc-chula/intania-888-backend/internal/model"
	"github.com/gofiber/fiber/v2"
)

type contractService struct {
	calls int
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

func (*contractService) VoidBill(string, string, string) (*model.BillHeadDto, error) {
	return nil, nil
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
