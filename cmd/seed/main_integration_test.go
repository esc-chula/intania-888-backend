//go:build integration

package main

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/esc-chula/intania-888-backend/internal/domain/sporttype"
	"github.com/esc-chula/intania-888-backend/internal/testutil"
)

func TestCatalogueReseedPreservesRenamesAndRestoresDeletedDefaults(t *testing.T) {
	p, err := testutil.OpenPostgres(os.Getenv("INTANIA888_TEST_DATABASE_URL"))
	if err != nil {
		if errors.Is(err, testutil.ErrMissingTestDatabaseURL) {
			t.Skip(err)
		}
		t.Fatal(err)
	}
	defer func() {
		if err := p.Close(); err != nil {
			t.Error(err)
		}
	}()
	if err := p.ResetAndMigrate(); err != nil {
		t.Fatal(err)
	}
	if err := seedCatalogue(p.DB); err != nil {
		t.Fatal(err)
	}

	service := sporttype.NewService(sporttype.NewGORMRepository(p.DB), nil)
	ctx := context.Background()
	if _, err := service.UpdateSportType(ctx, sporttype.Running, "Renamed running"); err != nil {
		t.Fatal(err)
	}
	if err := service.DeleteSportType(ctx, sporttype.TugOfWar); err != nil {
		t.Fatal(err)
	}
	if _, err := service.CreateSportType(ctx, sporttype.SportType{ID: "CUSTOM", Title: "Custom sport"}); err != nil {
		t.Fatal(err)
	}
	if err := seedCatalogue(p.DB); err != nil {
		t.Fatal(err)
	}
	for id, title := range map[string]string{
		sporttype.Running:  "Renamed running",
		sporttype.TugOfWar: "ชักเย่อ",
		"CUSTOM":           "Custom sport",
	} {
		row, err := service.GetSportType(ctx, id)
		if err != nil || row.Title != title {
			t.Fatalf("reseed %s = %+v; err=%v", id, row, err)
		}
	}
	rows, err := service.GetAllSportTypes(ctx)
	if err != nil || len(rows) != 14 {
		t.Fatalf("reseed duplicated or lost entries: count=%d err=%v", len(rows), err)
	}
}
