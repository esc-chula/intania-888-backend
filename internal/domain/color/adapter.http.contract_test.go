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
	"github.com/esc-chula/intania-888-backend/internal/value"
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

func (s *contractService) GetCoinRanking(context.Context) ([]*CoinRank, error) { return nil, s.err }

func (s *contractService) GetPredictionRanking(context.Context) ([]*PredictionRank, error) {
	return nil, s.err
}

func (s *contractService) GetGroupStageTable(ctx context.Context, typeID, groupID string) ([]*Leaderboard, error) {
	s.context, s.typeID, s.groupID = ctx, typeID, groupID
	s.invocations++

	return s.rows, s.err
}

func TestColorHTTPPreservesRowsEmptyListsFiltersAndContext(t *testing.T) {
	const typeID = "sport_TYPE-1"
	const groupID = "group_1-all"
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
		{"invalid type", "/colors/leaderboards?type_id=%E0%B8%81", nil, 400, "INVALID_REQUEST", 0},
		{"invalid group", "/colors/group-stage?group_id=%80", nil, 400, "INVALID_REQUEST", 0},
		{"empty type", "/colors/leaderboards?type_id=", nil, 400, "INVALID_REQUEST", 0},
		{"empty group", "/colors/group-stage?group_id=", nil, 400, "INVALID_REQUEST", 0},
		{"long type", "/colors/leaderboards?type_id=" + strings.Repeat("a", 101), nil, 400, "INVALID_REQUEST", 0},
		{"long group", "/colors/group-stage?group_id=" + strings.Repeat("a", 101), nil, 400, "INVALID_REQUEST", 0},
		{"space type_id", "/colors/leaderboards?type_id=%20S%20", nil, 400, "INVALID_REQUEST", 0},
		{"space type_id", "/colors/group-stage?type_id=%20S%20", nil, 400, "INVALID_REQUEST", 0},
		{"space group_id", "/colors/group-stage?group_id=%20S%20", nil, 400, "INVALID_REQUEST", 0},
		{"slash type_id", "/colors/leaderboards?type_id=a%2Fb", nil, 400, "INVALID_REQUEST", 0},
		{"slash type_id", "/colors/group-stage?type_id=a%2Fb", nil, 400, "INVALID_REQUEST", 0},
		{"slash group_id", "/colors/group-stage?group_id=a%2Fb", nil, 400, "INVALID_REQUEST", 0},
		{"question type_id", "/colors/leaderboards?type_id=a%3Fb", nil, 400, "INVALID_REQUEST", 0},
		{"question type_id", "/colors/group-stage?type_id=a%3Fb", nil, 400, "INVALID_REQUEST", 0},
		{"question group_id", "/colors/group-stage?group_id=a%3Fb", nil, 400, "INVALID_REQUEST", 0},
		{"percent type_id", "/colors/leaderboards?type_id=a%25b", nil, 400, "INVALID_REQUEST", 0},
		{"percent type_id", "/colors/group-stage?type_id=a%25b", nil, 400, "INVALID_REQUEST", 0},
		{"percent group_id", "/colors/group-stage?group_id=a%25b", nil, 400, "INVALID_REQUEST", 0},
		{"NUL type_id", "/colors/leaderboards?type_id=a%00b", nil, 400, "INVALID_REQUEST", 0},
		{"NUL type_id", "/colors/group-stage?type_id=a%00b", nil, 400, "INVALID_REQUEST", 0},
		{"NUL group_id", "/colors/group-stage?group_id=a%00b", nil, 400, "INVALID_REQUEST", 0},
		{"control type_id", "/colors/leaderboards?type_id=a%7Fb", nil, 400, "INVALID_REQUEST", 0},
		{"control type_id", "/colors/group-stage?type_id=a%7Fb", nil, 400, "INVALID_REQUEST", 0},
		{"control group_id", "/colors/group-stage?group_id=a%7Fb", nil, 400, "INVALID_REQUEST", 0},
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

type fakeRankingRepo struct {
	Repository
	coins       []*CoinStanding
	predictions []*PredictionStanding
	err         error
	calls       int
}

func (r *fakeRankingRepo) CoinStandings(context.Context) ([]*CoinStanding, error) {
	r.calls++

	return r.coins, r.err
}

func (r *fakeRankingRepo) PredictionStandings(context.Context) ([]*PredictionStanding, error) {
	r.calls++

	return r.predictions, r.err
}

func money(minor int64) value.Money { return value.MustMoneyFromMinor(minor) }

func TestCoinRankingOrdersByCoinThenColorID(t *testing.T) {
	repo := &fakeRankingRepo{coins: []*CoinStanding{
		{ID: "C", TotalCoin: money(100)},
		{ID: "B", TotalCoin: money(500)},
		{ID: "A", TotalCoin: money(500)},
		{ID: "D", TotalCoin: money(0)},
	}}
	got, err := NewService(repo, zap.NewNop()).GetCoinRanking(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for i, id := range []string{"A", "B", "C", "D"} {
		if got[i].ID != id || got[i].Rank != i+1 {
			t.Fatalf("row %d = %s rank %d, want %s rank %d", i, got[i].ID, got[i].Rank, id, i+1)
		}
	}
}

func TestPredictionRankingOrdersByCorrectThenFewerWrongThenColorID(t *testing.T) {
	repo := &fakeRankingRepo{predictions: []*PredictionStanding{
		{ID: "D", Correct: 1, Wrong: 0},
		{ID: "C", Correct: 5, Wrong: 5},
		{ID: "B", Correct: 5, Wrong: 1},
		{ID: "A", Correct: 5, Wrong: 1},
		{ID: "E"},
	}}
	got, err := NewService(repo, zap.NewNop()).GetPredictionRanking(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for i, id := range []string{"A", "B", "C", "D", "E"} {
		if got[i].ID != id || got[i].Rank != i+1 {
			t.Fatalf("row %d = %s rank %d, want %s", i, got[i].ID, got[i].Rank, id)
		}
	}
	if got[2].Total != 10 || got[2].Accuracy != 50 {
		t.Fatalf("C total/accuracy = %d/%v, want 10/50", got[2].Total, got[2].Accuracy)
	}
	if got[4].Total != 0 || got[4].Accuracy != 0 {
		t.Fatalf("E total/accuracy = %d/%v, want 0/0", got[4].Total, got[4].Accuracy)
	}
}

func TestRankingReturnsRepositoryError(t *testing.T) {
	service := NewService(&fakeRankingRepo{err: errors.New("db")}, zap.NewNop())
	if _, err := service.GetCoinRanking(context.Background()); err == nil {
		t.Fatal("coin ranking error = nil")
	}
	if _, err := service.GetPredictionRanking(context.Background()); err == nil {
		t.Fatal("prediction ranking error = nil")
	}
}

func getRanking(t *testing.T, service ServicePort, path string) (int, string) {
	t.Helper()
	app := fiber.New(fiber.Config{ErrorHandler: apierror.ErrorHandler(zap.NewNop())})
	NewHTTPHandler(service).RegisterRoutes(app)
	response, err := app.Test(httptest.NewRequest("GET", path, nil))
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

	return response.StatusCode, string(body)
}

type rankingHTTPService struct {
	contractService
	coins       []*CoinRank
	predictions []*PredictionRank
	err         error
}

func (s *rankingHTTPService) GetCoinRanking(context.Context) ([]*CoinRank, error) {
	return s.coins, s.err
}

func (s *rankingHTTPService) GetPredictionRanking(context.Context) ([]*PredictionRank, error) {
	return s.predictions, s.err
}

func TestRankingHTTPResponseShapes(t *testing.T) {
	service := &rankingHTTPService{
		coins: []*CoinRank{{Rank: 1, CoinStanding: CoinStanding{ID: "YELLOW", Title: "สีเหลือง", TotalCoin: money(1234500)}}},
		predictions: []*PredictionRank{{
			Rank: 1, PredictionStanding: PredictionStanding{ID: "YELLOW", Correct: 2, Wrong: 1}, Total: 3, Accuracy: 66.67,
		}},
	}
	tests := []struct{ path, want string }{
		{"/colors/leaderboards/coins", `[{"rank":1,"id":"YELLOW","title":"สีเหลือง","total_coin":"12345.00"}]`},
		{"/colors/leaderboards/predictions", `[{"rank":1,"id":"YELLOW","correct":2,"wrong":1,"total":3,"accuracy":66.67}]`},
	}
	for _, test := range tests {
		if status, body := getRanking(t, service, test.path); status != 200 || body != test.want {
			t.Fatalf("%s = %d %s, want 200 %s", test.path, status, body, test.want)
		}
	}
}

func TestRankingHTTPEmptyListsAndErrors(t *testing.T) {
	for _, path := range []string{"/colors/leaderboards/coins", "/colors/leaderboards/predictions"} {
		if status, body := getRanking(t, &rankingHTTPService{}, path); status != 200 || body != "[]" {
			t.Fatalf("%s empty = %d %s, want 200 []", path, status, body)
		}
		status, body := getRanking(t, &rankingHTTPService{err: errors.New("database secret")}, path)
		if status != 500 {
			t.Fatalf("%s error status = %d, want 500", path, status)
		}
		if strings.Contains(body, "database secret") {
			t.Fatalf("%s leaked the internal error: %s", path, body)
		}
	}
}
