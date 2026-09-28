package event

import (
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/esc-chula/intania-888-backend/internal/apierror"
	"github.com/esc-chula/intania-888-backend/internal/model"
	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

type errorEventService struct {
	redeemErr   error
	deleteErr   error
	spinErr     error
	scheduleErr error
}

func (s errorEventService) RedeemDailyReward(*model.UserDto) error { return s.redeemErr }
func (s errorEventService) GetDailyRewardSchedule() (*model.DailyRewardScheduleResponse, error) {
	return nil, s.scheduleErr
}
func (s errorEventService) SpinSlotMachine(*model.UserDto, model.Money) (map[string]interface{}, error) {
	return nil, s.spinErr
}
func (s errorEventService) SetDailyReward(string, model.Money) error { return nil }
func (s errorEventService) DeleteDailyReward(string) error           { return s.deleteErr }
func (s errorEventService) UseStealToken(string, string, int) (*model.UseStealTokenResponseDto, error) {
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
			name: "daily claim conflict", method: "GET", path: "/events/redeem/daily",
			service:    errorEventService{redeemErr: ErrDailyRewardAlreadyClaimed},
			wantStatus: fiber.StatusConflict, wantCode: "DAILY_REWARD_ALREADY_CLAIMED",
		},
		{
			name: "missing reward override", method: "DELETE", path: "/events/daily-rewards/25-09-2026",
			service:    errorEventService{deleteErr: ErrDailyRewardOverrideNotFound},
			wantStatus: fiber.StatusNotFound, wantCode: "RESOURCE_NOT_FOUND",
		},
		{
			name: "insufficient balance", method: "POST", path: "/events/spin/slot?spendAmount=50.00",
			service:    errorEventService{spinErr: ErrInsufficientBalance},
			wantStatus: fiber.StatusUnprocessableEntity, wantCode: "INSUFFICIENT_BALANCE",
		},
		{
			name: "storage failure redacted", method: "GET", path: "/events/daily-rewards",
			service:    errorEventService{scheduleErr: errors.New("database secret")},
			wantStatus: fiber.StatusInternalServerError, wantCode: "INTERNAL_ERROR",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler := NewEventHttpHandler(test.service)
			app := fiber.New(fiber.Config{ErrorHandler: apierror.ErrorHandler(zap.NewNop())})
			app.Use(apierror.RequestID())
			app.Get("/events/redeem/daily", func(c *fiber.Ctx) error {
				c.Locals("user", &model.UserDto{Id: "user"})
				return handler.RedeemDailyReward(c)
			})
			app.Delete("/events/daily-rewards/:date", handler.DeleteDailyReward)
			app.Post("/events/spin/slot", func(c *fiber.Ctx) error {
				c.Locals("user", &model.UserDto{Id: "user"})
				return handler.SpinSlotMachine(c)
			})
			app.Get("/events/daily-rewards", handler.GetDailyRewardSchedule)

			response, err := app.Test(httptest.NewRequest(test.method, test.path, nil))
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
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
