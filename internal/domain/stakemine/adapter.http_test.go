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
	history      []HistoryResult
	stats        *StatsResult
}

func (s *fakeHTTPService) RevealTile(_ context.Context, _, _ string, input RevealInput) (*GameResult, string, error) {
	s.revealCalled = true
	s.index = input.Index

	return &GameResult{
		ID:         "game",
		BetAmount:  value.MustMoneyFromMinor(10000),
		Multiplier: value.MustRateFromMicro(1030000),
		Grid:       []TileResult{},
	}, "message", nil
}

func (s *fakeHTTPService) GetGameHistory(_ context.Context, _ string, limit, offset int) ([]HistoryResult, error) {
	s.limit = limit
	s.offset = offset

	return s.history, nil
}

func (s *fakeHTTPService) GetStats(context.Context, string) (*StatsResult, error) {
	return s.stats, nil
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

				if service.index != 0 || body.Game["bet_amount"] != "100.00" || body.Game["multiplier"] != "1.030000" {
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
	if response.StatusCode != fiber.StatusOK ||
		service.limit != 100 ||
		service.offset != 2 ||
		body.Data == nil ||
		body.Limit != 100 ||
		body.Offset != 2 {
		t.Fatalf("pagination/empty shape changed: service=%+v body=%+v", service, body)
	}
}

func TestHistoryReturnsStringMultiplierAndNumericPagination(t *testing.T) {
	service := &fakeHTTPService{history: []HistoryResult{{
		GameID:        "game",
		BetAmount:     value.MustMoneyFromMinor(10_000),
		FinalPayout:   value.MustMoneyFromMinor(10_300),
		Multiplier:    value.MustRateFromMicro(1_030_000),
		RevealedCount: 1,
	}}}
	handler := NewHTTPHandler(service)
	app := fiber.New(fiber.Config{ErrorHandler: apierror.ErrorHandler(zap.NewNop())})
	app.Get("/mines/history", func(c *fiber.Ctx) error {
		httpidentity.SetProfile(c, &identity.Profile{ID: "owner"})

		return handler.GetHistory(c)
	})

	response, err := app.Test(httptest.NewRequest("GET", "/mines/history", nil))
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		if err := response.Body.Close(); err != nil {
			t.Error(err)
		}
	}()

	var body struct {
		Data   []map[string]any `json:"data"`
		Limit  int              `json:"limit"`
		Offset int              `json:"offset"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != fiber.StatusOK || len(body.Data) != 1 || body.Limit != 20 || body.Offset != 0 {
		t.Fatalf("history response = %+v; status %d", body, response.StatusCode)
	}
	game := body.Data[0]
	if game["multiplier"] != "1.030000" ||
		game["bet_amount"] != "100.00" ||
		game["final_payout"] != "103.00" ||
		game["revealed_count"] != float64(1) {
		t.Fatalf("history decimal/count contract = %+v", game)
	}
}

func TestStatsKeepApproximateWinRateAndCountsAsNumbers(t *testing.T) {
	service := &fakeHTTPService{stats: &StatsResult{
		TotalGames:    3,
		GamesWon:      1,
		TotalWagered:  value.MustMoneyFromMinor(30_000),
		TotalWinnings: value.MustMoneyFromMinor(10_300),
		NetProfit:     value.NewSignedMoneyFromMinor(-19_700),
		WinRate:       100.0 / 3,
	}}
	handler := NewHTTPHandler(service)
	app := fiber.New(fiber.Config{ErrorHandler: apierror.ErrorHandler(zap.NewNop())})
	app.Get("/mines/stats", func(c *fiber.Ctx) error {
		httpidentity.SetProfile(c, &identity.Profile{ID: "owner"})

		return handler.GetStats(c)
	})

	response, err := app.Test(httptest.NewRequest("GET", "/mines/stats", nil))
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
	if response.StatusCode != fiber.StatusOK ||
		body["win_rate"] != 100.0/3 ||
		body["total_games"] != float64(3) ||
		body["games_won"] != float64(1) {
		t.Fatalf("statistical number contract = %+v; status %d", body, response.StatusCode)
	}
	if body["total_wagered"] != "300.00" || body["total_winnings"] != "103.00" || body["net_profit"] != "-197.00" {
		t.Fatalf("money contract changed: %+v", body)
	}
}
