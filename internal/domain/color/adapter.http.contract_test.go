package color

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
	rows        []*Leaderboard
	err         error
	context     context.Context
	typeID      string
	groupID     string
	invocations int
}

func (s *contractService) GetAllLeaderboards(ctx context.Context, typeID string) ([]*Leaderboard, error) {
	s.context, s.typeID = ctx, typeID
	s.invocations++

	return s.rows, s.err
}

func (s *contractService) GetGroupStageTable(ctx context.Context, typeID, groupID string) ([]*Leaderboard, error) {
	s.context, s.typeID, s.groupID = ctx, typeID, groupID
	s.invocations++

	return s.rows, s.err
}

func TestColorHTTPPreservesRowsEmptyListsFiltersAndContext(t *testing.T) {
	const typeID = "c2b3f5a1-5fd6-4fa7-b313-86c70e3a2790"
	const groupID = "c52ca5b8-5036-4fb5-af9f-a7a6e9ca091d"
	tests := []struct {
		name string
		path string
		rows []*Leaderboard
		want string
	}{
		{
			name: "leaderboard fields",
			path: "/colors/leaderboards?type_id=" + typeID,
			rows: []*Leaderboard{{
				ID:         "red",
				Title:      "Red",
				Won:        2,
				Drawn:      1,
				Lost:       3,
				TotalMatch: 6,
			}},
			want: `[{"id":"red","title":"Red","won":2,"drawn":1,"lost":3,"total_matches":6}]`,
		},
		{
			name: "group stage omits empty title",
			path: "/colors/group-stage?type_id=" + typeID + "&group_id=" + groupID,
			rows: []*Leaderboard{{ID: "blue"}},
			want: `[{"id":"blue","won":0,"drawn":0,"lost":0,"total_matches":0}]`,
		},
		{
			name: "empty leaderboard",
			path: "/colors/leaderboards",
			want: `[]`,
		},
		{
			name: "empty group stage",
			path: "/colors/group-stage",
			rows: []*Leaderboard{},
			want: `[]`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			service := &contractService{rows: test.rows}
			app := fiber.New(fiber.Config{ErrorHandler: apierror.ErrorHandler(zap.NewNop())})
			app.Use(func(c *fiber.Ctx) error {
				c.SetUserContext(ctx)

				return c.Next()
			})
			NewHTTPHandler(service).RegisterRoutes(app)

			response, err := app.Test(httptest.NewRequest("GET", test.path, nil))
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
			if response.StatusCode != fiber.StatusOK || string(body) != test.want {
				t.Fatalf("status/body = %d %s, want 200 %s", response.StatusCode, body, test.want)
			}
			if service.context != ctx || service.invocations != 1 {
				t.Fatal("request context or service invocation changed")
			}
			if strings.Contains(test.path, "type_id=") && service.typeID != typeID {
				t.Fatalf("type filter = %q", service.typeID)
			}
			if strings.Contains(test.path, "group_id=") && service.groupID != groupID {
				t.Fatalf("group filter = %q", service.groupID)
			}
		})
	}
}

func TestColorHTTPPreservesValidationAndDependencyErrorContract(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		err        error
		wantStatus int
		wantCode   string
		wantCalls  int
	}{
		{"invalid type", "/colors/leaderboards?type_id=invalid", nil, 400, "INVALID_REQUEST", 0},
		{"invalid group", "/colors/group-stage?group_id=invalid", nil, 400, "INVALID_REQUEST", 0},
		{"storage failure", "/colors/leaderboards", errors.New("database secret"), 500, "INTERNAL_ERROR", 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := &contractService{err: test.err}
			app := fiber.New(fiber.Config{ErrorHandler: apierror.ErrorHandler(zap.NewNop())})
			app.Use(apierror.RequestID())
			NewHTTPHandler(service).RegisterRoutes(app)

			response, err := app.Test(httptest.NewRequest("GET", test.path, nil))
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
			if response.StatusCode != test.wantStatus || body.Code != test.wantCode || service.invocations != test.wantCalls {
				t.Fatalf("status/body/calls = %d %+v %d", response.StatusCode, body, service.invocations)
			}
			if strings.Contains(body.Message, "database secret") ||
				body.RequestID == "" ||
				body.RequestID != response.Header.Get(apierror.RequestIDHeader) {
				t.Fatalf("unsafe or uncorrelated error response: %+v", body)
			}
		})
	}
}
