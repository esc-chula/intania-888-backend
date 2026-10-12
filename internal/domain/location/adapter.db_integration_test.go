//go:build integration

package location_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/esc-chula/intania-888-backend/internal/domain/location"
	"github.com/esc-chula/intania-888-backend/internal/domain/match"
	"github.com/esc-chula/intania-888-backend/internal/testutil"
)

func TestLocationPostgresCRUDAndMatchForeignKey(t *testing.T) {
	p, err := testutil.OpenPostgres(os.Getenv("INTANIA888_TEST_DATABASE_URL"))
	if err != nil {
		if errors.Is(err, testutil.ErrMissingTestDatabaseURL) {
			t.Skip(err)
		}
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := p.Close(); err != nil {
			t.Error(err)
		}
	})
	if err := p.ResetAndMigrate(); err != nil {
		t.Fatal(err)
	}
	if _, err := p.SQL.Exec(`INSERT INTO sport_types(id,title) VALUES('S','Sport')`); err != nil {
		t.Fatal(err)
	}

	service := location.NewService(location.NewGORMRepository(p.DB), nil)
	ctx := context.Background()
	row, err := service.CreateLocation(ctx, location.Location{
		ID:    "VENUE",
		Title: "  Venue  ",
	})
	if err != nil || row.Title != "Venue" {
		t.Fatalf("create = %+v, %v", row, err)
	}
	if _, err := service.CreateLocation(ctx, location.Location{
		ID:    "VENUE",
		Title: "Other",
	}); !errors.Is(err, location.ErrLocationConflict) {
		t.Fatalf("duplicate ID error = %v", err)
	}
	if _, err := service.UpdateLocation(ctx, "VENUE", " New venue "); err != nil {
		t.Fatal(err)
	}

	matchRepo := match.NewGORMRepository(p.DB)
	now := time.Now().UTC()
	if err := matchRepo.Create(ctx, &match.Snapshot{
		ID:         "MATCH",
		TypeID:     "S",
		LocationID: "MISSING",
		StartTime:  now,
		EndTime:    now.Add(time.Hour),
	}); !errors.Is(err, match.ErrInvalidLocation) {
		t.Fatalf("unknown location reference = %v", err)
	}
	if err := matchRepo.Create(ctx, &match.Snapshot{
		ID:         "MATCH",
		TypeID:     "S",
		LocationID: "VENUE",
		StartTime:  now,
		EndTime:    now.Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	if err := service.DeleteLocation(ctx, "VENUE"); !errors.Is(err, location.ErrLocationInUse) {
		t.Fatalf("delete referenced location = %v", err)
	}
	results, err := match.NewService(matchRepo, matchRepo, time.Now, func() string {
		return "unused"
	}, nil).GetMatch(ctx, "MATCH")
	if err != nil || results.LocationID != "VENUE" || results.LocationTitle != "New venue" {
		t.Fatalf("match location = %+v; err=%v", results, err)
	}
}
