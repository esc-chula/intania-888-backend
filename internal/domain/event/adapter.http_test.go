package event

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
	"github.com/esc-chula/intania-888-backend/internal/identity"
	"github.com/esc-chula/intania-888-backend/internal/value"
)

type errorEventService struct {
	redeemErr   error
	deleteErr   error
	spinErr     error
	scheduleErr error
}

func (s errorEventService) RedeemDailyReward(context.Context, string) error {
	return s.redeemErr
}

func (s errorEventService) GetDailyRewardSchedule(context.Context) (*DailyRewardSchedule, error) {
	return nil, s.scheduleErr
}

func (s errorEventService) SpinSlotMachine(context.Context, identity.Profile, value.Money) (*SpinResult, error) {
	return nil, s.spinErr
}

func (s errorEventService) SetDailyReward(context.Context, string, value.Money) error {
	return nil
}

func (s errorEventService) DeleteDailyReward(context.Context, string) error {
	return s.deleteErr
}

func (s errorEventService) UseStealToken(context.Context, string, string, int) (*StealResult, error) {
	return nil, nil
}

func TestEventFailuresUseSharedContract(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		path       string
		service    errorEventService
		wantStatus int
		wantCode   string
	}{
		{
			name:       "daily claim conflict",
			method:     "GET",
			path:       "/events/redeem/daily",
			service:    errorEventService{redeemErr: ErrDailyRewardAlreadyClaimed},
			wantStatus: fiber.StatusConflict,
			wantCode:   "DAILY_REWARD_ALREADY_CLAIMED",
		},
		{
			name:       "missing reward override",
			method:     "DELETE",
			path:       "/events/daily-rewards/25-09-2026",
			service:    errorEventService{deleteErr: ErrDailyRewardOverrideNotFound},
			wantStatus: fiber.StatusNotFound,
			wantCode:   "RESOURCE_NOT_FOUND",
		},
		{
			name:       "insufficient balance",
			method:     "POST",
			path:       "/events/spin/slot?spendAmount=50.00",
			service:    errorEventService{spinErr: ErrInsufficientBalance},
			wantStatus: fiber.StatusUnprocessableEntity,
			wantCode:   "INSUFFICIENT_BALANCE",
		},
		{
			name:       "storage failure redacted",
			method:     "GET",
			path:       "/events/daily-rewards",
			service:    errorEventService{scheduleErr: errors.New("database secret")},
			wantStatus: fiber.StatusInternalServerError,
			wantCode:   "INTERNAL_ERROR",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler := NewHTTPHandler(test.service)
			app := fiber.New(fiber.Config{ErrorHandler: apierror.ErrorHandler(zap.NewNop())})
			app.Use(apierror.RequestID())
			app.Get("/events/redeem/daily", func(c *fiber.Ctx) error {
				c.Locals("user", &identity.Profile{ID: "user"})

				return handler.RedeemDailyReward(c)
			})
			app.Delete("/events/daily-rewards/:date", handler.DeleteDailyReward)
			app.Post("/events/spin/slot", func(c *fiber.Ctx) error {
				c.Locals("user", &identity.Profile{ID: "user"})

				return handler.SpinSlotMachine(c)
			})
			app.Get("/events/daily-rewards", handler.GetDailyRewardSchedule)

			response, err := app.Test(httptest.NewRequest(test.method, test.path, nil))
			if err != nil {
				t.Fatal(err)
			}

			defer func() {
				if err := response.Body.Close(); err != nil {
					t.Error(err)
				}
			}()
			if response.StatusCode != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.StatusCode, test.wantStatus)
			}
			var body apierror.Response
			if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.Code != test.wantCode {
				t.Fatalf("code = %q, want %q", body.Code, test.wantCode)
			}
			if body.RequestID == "" || response.Header.Get(apierror.RequestIDHeader) != body.RequestID {
				t.Fatalf("request ID mismatch: body=%q header=%q", body.RequestID, response.Header.Get(apierror.RequestIDHeader))
			}
			if strings.Contains(body.Message, "database secret") {
				t.Fatalf("message exposed internal detail: %q", body.Message)
			}
		})
	}
}

func TestEventMoneyAndIndexPresenceValidation(t *testing.T) {
	tests := []struct {
		name, body string
		valid      bool
		kind       string
	}{
		{"explicit zero override", `{"amount":"0.00"}`, true, "reward"},
		{"missing override", `{}`, false, "reward"},
		{"null override", `{"amount":null}`, false, "reward"},
		{"numeric override", `{"amount":0}`, false, "reward"},
		{"unknown override field", `{"amount":"0.00","extra":true}`, false, "reward"},
		{"zero candidate", `{"token":"token","victim_index":0}`, true, "token"},
		{"missing candidate", `{"token":"token"}`, false, "token"},
		{"null candidate", `{"token":"token","victim_index":null}`, false, "token"},
		{"negative candidate", `{"token":"token","victim_index":-1}`, false, "token"},
		{"unknown token field", `{"token":"token","victim_index":0,"extra":1}`, false, "token"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var valid bool
			if test.kind == "reward" {
				var request SetDailyRewardRequest
				err := json.Unmarshal([]byte(test.body), &request)
				valid = err == nil && len(request.ValidateRequest()) == 0
			} else {
				var request UseStealTokenRequest
				err := json.Unmarshal([]byte(test.body), &request)
				valid = err == nil && len(request.ValidateRequest()) == 0
			}
			if valid != test.valid {
				t.Fatalf("valid=%v, want %v", valid, test.valid)
			}
		})
	}
}

func TestSlotResponsePreservesConditionalFields(t *testing.T) {
	for _, token := range []bool{false, true} {
		t.Run(map[bool]string{
			false: "coin reward",
			true:  "token reward",
		}[token], func(t *testing.T) {
			result := &SpinResult{
				Slots:  []string{"a", "b", "c"},
				Reward: value.MustMoneyFromMinor(75_00),
			}
			if token {
				result.Reward = value.Money{}
				result.StealToken = &TokenReward{
					Token:       "token",
					VictimCount: 3,
					Message:     "message",
				}
				result.Candidates = []CandidatePreview{{
					Index:  0,
					Name:   "victim",
					RoleID: "USER",
				}}
			}
			encoded, err := json.Marshal(spinToResponse(result))
			if err != nil {
				t.Fatal(err)
			}
			var response map[string]json.RawMessage
			if err := json.Unmarshal(encoded, &response); err != nil {
				t.Fatal(err)
			}
			if _, ok := response["stealToken"]; ok != token {
				t.Fatalf("token field=%v, want %v: %s", ok, token, encoded)
			}
			if _, ok := response["candidates"]; ok != token {
				t.Fatalf("candidates field=%v, want %v: %s", ok, token, encoded)
			}
			if _, ok := response["steal_token"]; ok {
				t.Fatalf("changed legacy token spelling: %s", encoded)
			}
			expectedReward := `"75.00"`
			if token {
				expectedReward = `"0.00"`
			}
			if string(response["reward"]) != expectedReward {
				t.Fatalf("reward=%s, want %s", response["reward"], expectedReward)
			}
		})
	}
}

func TestScheduleResponseEmptyOverridesAreArray(t *testing.T) {
	encoded, err := json.Marshal(scheduleToResponse(&DailyRewardSchedule{DefaultAmount: value.MustMoneyFromMinor(300_00)}))
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded) != `{"default_amount":"300.00","overrides":[]}` {
		t.Fatalf("schedule=%s", encoded)
	}
}
