package match

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/esc-chula/intania-888-backend/internal/apierror"
)

type matchFilterService struct {
	filter  *Filter
	context context.Context
}

func (*matchFilterService) CreateMatch(context.Context, *Input) error { return nil }

func (*matchFilterService) GetMatch(context.Context, string) (*Result, error) { return nil, nil }

func (*matchFilterService) GetTime() (string, error) { return "", nil }

func (s *matchFilterService) GetAllMatches(ctx context.Context, filter *Filter) ([]*Result, error) {
	s.filter = filter
	s.context = ctx
	return []*Result{}, nil
}

func (*matchFilterService) UpdateMatchScore(context.Context, string, *ScoreInput) error { return nil }

func (*matchFilterService) SetResult(context.Context, string, *ResultInput) error { return nil }

func (*matchFilterService) UpdateMatch(context.Context, string, *Input) error { return nil }

func (*matchFilterService) DeleteMatch(context.Context, string) error { return nil }

func TestGetAllMatchesAcceptsOpaqueTypeID(t *testing.T) {
	service := &matchFilterService{}
	handler := NewHTTPHandler(service)
	app := fiber.New(fiber.Config{ErrorHandler: apierror.ErrorHandler(nil)})
	ctx := context.WithValue(context.Background(), contextKey{}, "http request")
	app.Use(func(c *fiber.Ctx) error { c.SetUserContext(ctx); return c.Next() })
	app.Get("/matches", handler.GetAllMatches)

	response, err := app.Test(httptest.NewRequest("GET", "/matches?typeId=S", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := response.Body.Close(); err != nil {
			t.Error(err)
		}
	}()

	if response.StatusCode != fiber.StatusOK {
		t.Fatalf("status = %d; want %d", response.StatusCode, fiber.StatusOK)
	}
	if service.context != ctx {
		t.Fatal("HTTP request context was not propagated to the use case")
	}
	if service.filter == nil || service.filter.TypeID != "S" {
		t.Fatalf("service filter = %#v; want type ID S", service.filter)
	}
}
