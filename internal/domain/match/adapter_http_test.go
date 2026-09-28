package match

import (
	"net/http/httptest"
	"testing"

	"github.com/esc-chula/intania-888-backend/internal/apierror"
	"github.com/esc-chula/intania-888-backend/internal/model"
	"github.com/gofiber/fiber/v2"
)

type matchFilterService struct {
	filter *model.MatchFilter
}

func (*matchFilterService) CreateMatch(*model.MatchDto) error { return nil }

func (*matchFilterService) GetMatch(string) (*model.MatchDto, error) { return nil, nil }

func (*matchFilterService) GetTime() (string, error) { return "", nil }

func (s *matchFilterService) GetAllMatches(filter *model.MatchFilter) ([]*model.MatchDto, error) {
	s.filter = filter
	return []*model.MatchDto{}, nil
}

func (*matchFilterService) UpdateMatchScore(string, *model.ScoreDto) error { return nil }

func (*matchFilterService) SetResult(string, *model.MatchResultRequest) error { return nil }

func (*matchFilterService) UpdateMatch(string, *model.MatchDto) error { return nil }

func (*matchFilterService) DeleteMatch(string) error { return nil }

func TestGetAllMatchesAcceptsOpaqueTypeID(t *testing.T) {
	service := &matchFilterService{}
	handler := NewMatchHttpHandler(service)
	app := fiber.New(fiber.Config{ErrorHandler: apierror.ErrorHandler(nil)})
	app.Get("/matches", handler.GetAllMatches)

	response, err := app.Test(httptest.NewRequest("GET", "/matches?typeId=S", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()

	if response.StatusCode != fiber.StatusOK {
		t.Fatalf("status = %d; want %d", response.StatusCode, fiber.StatusOK)
	}
	if service.filter == nil || service.filter.TypeId != "S" {
		t.Fatalf("service filter = %#v; want type ID S", service.filter)
	}
}
