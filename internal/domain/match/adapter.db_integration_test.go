//go:build integration

package match

import (
	"context"
	"errors"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/esc-chula/intania-888-backend/internal/testutil"
)

func TestMatchPostgresScheduleFilters(t *testing.T) {
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

	for _, statement := range []string{
		`INSERT INTO colors(id,title) VALUES('A','A'),('B','B')`,
		`INSERT INTO sport_types(id,title) VALUES('S','Sport'),('OTHER','Other sport')`,
		`INSERT INTO locations(id,title) VALUES('L','Location')`,
	} {
		if _, err := p.SQL.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}

	now := time.Date(2026, time.October, 7, 12, 0, 0, 0, time.UTC)
	winner := "A"
	teamB := "B"
	score := 1
	fixtures := []struct {
		id     string
		end    time.Time
		winner *string
		draw   bool
		score  *int
	}{
		{"future", now.Add(time.Hour), nil, false, nil},
		{"scored", now.Add(time.Hour), nil, false, &score},
		{"early_winner", now.Add(time.Hour), &winner, false, nil},
		{"early_draw", now.Add(time.Hour), nil, true, nil},
		{"expired", now.Add(-time.Hour), nil, false, nil},
		{"expired_winner", now.Add(-time.Hour), &winner, false, nil},
		{"expired_draw", now.Add(-time.Hour), nil, true, nil},
		{"boundary", now, nil, false, nil},
	}
	repo := NewGORMRepository(p.DB)
	ctx := context.Background()

	// Insert in reverse start-time order to verify repository ordering as well as membership.
	for i := len(fixtures) - 1; i >= 0; i-- {
		fixture := fixtures[i]
		for _, typeID := range []string{"S", "OTHER"} {
			if err := repo.Create(ctx, &Snapshot{
				ID:         typeID + "_" + fixture.id,
				TeamAID:    &winner,
				TeamBID:    &teamB,
				TeamAScore: fixture.score,
				TeamBScore: fixture.score,
				WinnerID:   fixture.winner,
				IsDraw:     fixture.draw,
				TypeID:     typeID,
				LocationID: "L",
				StartTime:  now.Add(-24*time.Hour + time.Duration(i)*time.Minute),
				EndTime:    fixture.end,
			}); err != nil {
				t.Fatal(err)
			}
		}
	}

	all := []string{"future", "scored", "early_winner", "early_draw", "expired", "expired_winner", "expired_draw", "boundary"}
	for _, tc := range []struct {
		name     string
		schedule ScheduleFilter
		want     []string
	}{
		{"unfiltered", "", all},
		{"schedule", Schedule, []string{"future", "scored"}},
		{"result", ScheduleResult, all[2:]},
	} {
		for _, typeID := range []string{"", "S"} {
			t.Run(tc.name+"/type="+typeID, func(t *testing.T) {
				rows, err := repo.GetAll(ctx, &Filter{TypeID: typeID, Schedule: tc.schedule}, now)
				if err != nil {
					t.Fatal(err)
				}

				var want []string
				for _, id := range tc.want {
					want = append(want, "S_"+id)
					if typeID == "" {
						want = append(want, "OTHER_"+id)
					}
				}
				got := make([]string, len(rows))
				for i, row := range rows {
					got[i] = row.ID
					if i > 0 && row.StartTime.Before(rows[i-1].StartTime) {
						t.Fatal("matches are not ordered by ascending start time")
					}
				}
				slices.Sort(got)
				slices.Sort(want)
				if !slices.Equal(got, want) {
					t.Fatalf("match IDs = %v; want %v", got, want)
				}
			})
		}
	}

	rows, err := repo.GetAll(ctx, nil, now)
	if err != nil || len(rows) != 2*len(fixtures) {
		t.Fatalf("nil filter returned %d matches, error %v", len(rows), err)
	}
}
