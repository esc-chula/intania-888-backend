package match

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/esc-chula/intania-888-backend/internal/apierror"
	"github.com/esc-chula/intania-888-backend/internal/value"
)

type matchFilterService struct {
	filter  *Filter
	context context.Context
	result  *Result
}

func (*matchFilterService) CreateMatch(context.Context, *Input) error { return nil }

func (s *matchFilterService) GetMatch(context.Context, string) (*Result, error) {
	return s.result, nil
}

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

func TestGetMatchReturnsStringRatesAndNumericScores(t *testing.T) {
	zero := 0
	service := &matchFilterService{result: &Result{
		ID:         "match",
		TeamARate:  value.MustRateFromMicro(1_234_567),
		TeamBRate:  value.Rate{},
		TeamAScore: &zero,
	}}
	handler := NewHTTPHandler(service)
	app := fiber.New(fiber.Config{ErrorHandler: apierror.ErrorHandler(nil)})
	app.Get("/matches/:id", handler.GetMatch)

	response, err := app.Test(httptest.NewRequest("GET", "/matches/match", nil))
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
	if response.StatusCode != fiber.StatusOK || body["team_a_rate"] != "1.234567" || body["team_b_rate"] != "0.000000" {
		t.Fatalf("rate contract = %+v; status %d", body, response.StatusCode)
	}
	if body["team_a_score"] != float64(0) || body["team_b_score"] != nil {
		t.Fatalf("score number/null contract changed: %+v", body)
	}
}
