package sporttype

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"

	"github.com/esc-chula/intania-888-backend/internal/apierror"
)

type contractService struct {
	ServicePort
	rows    []*SportType
	err     error
	context context.Context
}

func (s *contractService) GetAllSportTypes(ctx context.Context) ([]*SportType, error) {
	s.context = ctx
	return s.rows, s.err
}

func TestSportTypeHTTPPreservesCatalogueEmptyListsAndContext(t *testing.T) {
	tests := []struct {
		name string
		rows []*SportType
		want string
	}{
		{"catalogue", []*SportType{{ID: "football", Title: "Football"}, {ID: "empty"}}, `[{"id":"football","title":"Football"},{"id":"empty","title":""}]`},
		{"nil catalogue", nil, `[]`},
		{"empty catalogue", []*SportType{}, `[]`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			service := &contractService{rows: test.rows}
			app := fiber.New(fiber.Config{ErrorHandler: apierror.ErrorHandler(zap.NewNop())})
			NewHTTPHandler(service).RegisterRoutes(app, func(c *fiber.Ctx) error {
				c.SetUserContext(ctx)
				return c.Next()
			}, func(c *fiber.Ctx) error { return c.Next() })
			response, err := app.Test(httptest.NewRequest("GET", "/sport-types", nil))
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := response.Body.Close(); err != nil {
					t.Error(err)
				}
			}()
			body, err := io.ReadAll(response.Body)
			if err != nil {
				t.Fatal(err)
			}
			if response.StatusCode != 200 || string(body) != test.want || service.context != ctx {
				t.Fatalf("status/body/context = %d %s %v", response.StatusCode, body, service.context)
			}
		})
	}
}

func TestSportTypeHTTPRedactsDependencyErrorsAndCorrelatesRequest(t *testing.T) {
	service := &contractService{err: errors.New("database secret")}
	app := fiber.New(fiber.Config{ErrorHandler: apierror.ErrorHandler(zap.NewNop())})
	app.Use(apierror.RequestID())
	NewHTTPHandler(service).RegisterRoutes(app, func(c *fiber.Ctx) error { return c.Next() }, func(c *fiber.Ctx) error { return c.Next() })
	response, err := app.Test(httptest.NewRequest("GET", "/sport-types", nil))
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
	if response.StatusCode != 500 || body.Code != "INTERNAL_ERROR" || strings.Contains(body.Message, "database secret") || body.RequestID == "" || body.RequestID != response.Header.Get(apierror.RequestIDHeader) {
		t.Fatalf("unsafe or changed error response: status=%d body=%+v", response.StatusCode, body)
	}
}
