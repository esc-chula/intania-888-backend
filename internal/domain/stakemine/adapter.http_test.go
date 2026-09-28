package stakemine

import (
	"context"
	"encoding/json"
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
	revealCalled bool
	index        int
	limit        int
	offset       int
}

func (s *fakeHTTPService) RevealTile(_ context.Context, _, _ string, input RevealInput) (*GameResult, string, error) {
	s.revealCalled = true
	s.index = input.Index
	return &GameResult{ID: "game", BetAmount: value.MustMoneyFromMinor(10000), Multiplier: value.MustRateFromMicro(1030000), Grid: []TileResult{}}, "message", nil
}

func (s *fakeHTTPService) GetGameHistory(_ context.Context, _ string, limit, offset int) ([]HistoryResult, error) {
	s.limit = limit
	s.offset = offset
	return []HistoryResult{}, nil
}

func TestRevealTilePresenceAndZeroIndex(t *testing.T) {
	tests := []struct {
		name, body string
		wantStatus int
		wantCalled bool
	}{
		{"zero", `{"index":0}`, 200, true},
		{"missing", `{}`, 400, false},
		{"null", `{"index":null}`, 400, false},
		{"negative", `{"index":-1}`, 400, false},
		{"too large", `{"index":16}`, 400, false},
		{"unknown field", `{"index":0,"extra":true}`, 400, false},
		{"case changed field", `{"Index":0}`, 400, false},
		{"multiple objects", `{"index":0}{"index":1}`, 400, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := &fakeHTTPService{}
			app := fiber.New(fiber.Config{ErrorHandler: apierror.ErrorHandler(zap.NewNop())})
			handler := NewHTTPHandler(service)
			app.Post("/mines/:id/reveal", func(c *fiber.Ctx) error {
				httpidentity.SetProfile(c, &identity.Profile{ID: "owner"})
				return handler.RevealTile(c)
			})
			request := httptest.NewRequest("POST", "/mines/game/reveal", strings.NewReader(test.body))
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
			if response.StatusCode != test.wantStatus || service.revealCalled != test.wantCalled {
				t.Fatalf("status/called = %d %t", response.StatusCode, service.revealCalled)
			}
			if test.wantCalled {
				var body struct {
					Message string         `json:"message"`
					Game    map[string]any `json:"game"`
				}
				if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				if service.index != 0 || body.Game["bet_amount"] != "100.00" || body.Game["multiplier"] != 1.03 {
					t.Fatalf("success payload changed: %+v", body)
				}
				if _, exists := body.Game["completed_at"]; exists {
					t.Fatal("nil completed_at must remain omitted")
				}
			}
		})
	}
}

func TestHistoryPreservesLimitClampAndEmptyArray(t *testing.T) {
	service := &fakeHTTPService{}
	handler := NewHTTPHandler(service)
	app := fiber.New(fiber.Config{ErrorHandler: apierror.ErrorHandler(zap.NewNop())})
	app.Get("/mines/history", func(c *fiber.Ctx) error {
		httpidentity.SetProfile(c, &identity.Profile{ID: "owner"})
		return handler.GetHistory(c)
	})
	response, err := app.Test(httptest.NewRequest("GET", "/mines/history?limit=999&offset=2", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := response.Body.Close(); err != nil {
			t.Error(err)
		}
	}()
	var body HistoryListResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != 200 || service.limit != 100 || service.offset != 2 || body.Data == nil || body.Limit != 100 || body.Offset != 2 {
		t.Fatalf("pagination/empty shape changed: service=%+v body=%+v", service, body)
	}
}
