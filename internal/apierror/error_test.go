package apierror_test

import (
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"

	"github.com/esc-chula/intania-888-backend/internal/apierror"
	"github.com/esc-chula/intania-888-backend/internal/domain/match"
	"github.com/esc-chula/intania-888-backend/internal/domain/stakemine"
)

func TestErrorHandlerMapsWrappedAndUnknownErrors(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		status   int
		code     string
		message  string
		mustHide string
	}{
		{
			name:   "wrapped typed conflict",
			err:    fmtError(apierror.Wrap(errors.New("database detail"), fiber.StatusConflict, "DAILY_REWARD_ALREADY_CLAIMED", "Daily reward already claimed")),
			status: fiber.StatusConflict, code: "DAILY_REWARD_ALREADY_CLAIMED", message: "Daily reward already claimed", mustHide: "database detail",
		},
		{
			name:   "unknown error is redacted",
			err:    errors.New("private database password"),
			status: fiber.StatusInternalServerError, code: "INTERNAL_ERROR", message: "Internal server error", mustHide: "private database password",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app := fiber.New(fiber.Config{ErrorHandler: apierror.ErrorHandler(zap.NewNop())})
			app.Use(apierror.RequestID())
			app.Get("/api/v1/test", func(c *fiber.Ctx) error { return test.err })
			response, err := app.Test(httptest.NewRequest("GET", "/api/v1/test", nil))
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := response.Body.Close(); err != nil {
					t.Error(err)
				}
			}()
			if response.StatusCode != test.status {
				t.Fatalf("status = %d, want %d", response.StatusCode, test.status)
			}
			var body apierror.Response
			if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.Code != test.code || body.Message != test.message {
				t.Fatalf("body = %#v", body)
			}
			if body.RequestID == "" || response.Header.Get(apierror.RequestIDHeader) != body.RequestID {
				t.Fatalf("request ID body/header mismatch: %#v, %q", body, response.Header.Get(apierror.RequestIDHeader))
			}
			encoded, err := json.Marshal(body)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(encoded), test.mustHide) {
				t.Fatalf("response exposed internal error: %s", encoded)
			}
		})
	}
}

type validRequest struct {
	Index *int `json:"index"`
}

func (r validRequest) ValidateRequest() map[string]string {
	if r.Index == nil || *r.Index < 0 || *r.Index > 15 {
		return map[string]string{"index": "must be between 0 and 15"}
	}
	return nil
}

func TestBindJSONRejectsUnknownAndTrailingValuesAndAcceptsZero(t *testing.T) {
	tests := []struct {
		name string
		body string
		ok   bool
	}{
		{name: "valid zero", body: `{"index":0}`, ok: true},
		{name: "unknown field", body: `{"index":0,"extra":true}`},
		{name: "trailing value", body: `{"index":0} {}`},
		{name: "missing index", body: `{}`},
		{name: "out of range", body: `{"index":16}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app := fiber.New(fiber.Config{ErrorHandler: apierror.ErrorHandler(zap.NewNop())})
			app.Post("/", func(c *fiber.Ctx) error {
				var request validRequest
				return apierror.BindJSON(c, &request)
			})
			response, err := app.Test(httptest.NewRequest("POST", "/", strings.NewReader(test.body)))
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := response.Body.Close(); err != nil {
					t.Error(err)
				}
			}()
			if test.ok && response.StatusCode != fiber.StatusOK {
				t.Fatalf("status = %d, want 200", response.StatusCode)
			}
			if !test.ok && response.StatusCode == fiber.StatusOK {
				t.Fatal("invalid request was accepted")
			}
		})
	}
}

func TestRegisteredNumericRequestsDistinguishZeroFromMissing(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		body       string
		wantStatus int
		wantField  string
	}{
		{name: "score zero is valid", path: "/score", body: `{"team_a_score":0,"team_b_score":0}`, wantStatus: fiber.StatusNoContent},
		{name: "score omission is invalid", path: "/score", body: `{"team_b_score":0}`, wantStatus: fiber.StatusBadRequest, wantField: "team_a_score"},
		{name: "score rejects extra field", path: "/score", body: `{"team_a_score":0,"team_b_score":1,"unexpected":2}`, wantStatus: fiber.StatusBadRequest, wantField: "body"},
		{name: "tile zero is valid", path: "/tile", body: `{"index":0}`, wantStatus: fiber.StatusNoContent},
		{name: "tile omission is invalid", path: "/tile", body: `{}`, wantStatus: fiber.StatusBadRequest, wantField: "index"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app := fiber.New(fiber.Config{ErrorHandler: apierror.ErrorHandler(zap.NewNop())})
			app.Use(apierror.RequestID())
			app.Post("/score", func(c *fiber.Ctx) error {
				var request match.ScoreDTO
				if err := apierror.BindJSON(c, &request); err != nil {
					return err
				}
				return c.SendStatus(fiber.StatusNoContent)
			})
			app.Post("/tile", func(c *fiber.Ctx) error {
				var request stakemine.RevealTileRequest
				if err := apierror.BindJSON(c, &request); err != nil {
					return err
				}
				return c.SendStatus(fiber.StatusNoContent)
			})

			response, err := app.Test(httptest.NewRequest("POST", test.path, strings.NewReader(test.body)))
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
			if test.wantField == "" {
				return
			}
			var body apierror.Response
			if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.Code != "INVALID_REQUEST" || body.Details[test.wantField] == "" {
				t.Fatalf("validation error = %#v", body)
			}
		})
	}
}

func fmtError(err error) error { return errors.Join(errors.New("route wrapper"), err) }
