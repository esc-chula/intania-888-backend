//go:build integration

package sporttype

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"

	"github.com/esc-chula/intania-888-backend/internal/testutil"
)

func sportTypePostgres(t *testing.T) *testutil.Postgres {
	t.Helper()
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
	return p
}

func execSportSQL(t *testing.T, p *testutil.Postgres, statements ...string) {
	t.Helper()
	for _, statement := range statements {
		if _, err := p.SQL.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
}

var sportReferences = []struct{ name, table, insert string }{
	{"match", "matches", `INSERT INTO matches(id,type_id,start_time,end_time) VALUES($1,$1,now(),now())`},
	{"tournament group", "group_heads", `INSERT INTO group_heads(id,type_id,title) VALUES($1,$1,'Group')`},
	{"stage", "group_stages", `INSERT INTO group_stages(id,type_id,color_id) VALUES($1,$1,'A')`},
}

func TestSportTypePostgresCRUDAndSafeDeletion(t *testing.T) {
	p := sportTypePostgres(t)
	execSportSQL(t, p, `INSERT INTO colors(id,title) VALUES('A','A')`)
	service := NewService(NewGORMRepository(p.DB), nil)
	ctx := context.Background()

	for _, id := range []string{"UNUSED", "SAME_TITLE"} {
		row, err := service.CreateSportType(ctx, SportType{ID: id, Title: " Sport "})
		if err != nil || row.ID != id || row.Title != "Sport" {
			t.Fatalf("create = %+v %v", row, err)
		}
	}
	if _, err := service.CreateSportType(ctx, SportType{ID: "UNUSED", Title: "Other"}); !errors.Is(err, ErrSportTypeConflict) {
		t.Fatalf("duplicate ID = %v", err)
	}
	row, err := service.UpdateSportType(ctx, "UNUSED", " Renamed ")
	if err != nil || row.ID != "UNUSED" || row.Title != "Renamed" {
		t.Fatalf("rename = %+v %v", row, err)
	}
	row, err = service.GetSportType(ctx, "UNUSED")
	if err != nil || row.Title != "Renamed" {
		t.Fatalf("read renamed = %+v %v", row, err)
	}
	if err := service.DeleteSportType(ctx, "UNUSED"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.GetSportType(ctx, "UNUSED"); !errors.Is(err, ErrSportTypeNotFound) {
		t.Fatalf("read deleted = %v", err)
	}
	if _, err := service.UpdateSportType(ctx, "UNUSED", "Sport"); !errors.Is(err, ErrSportTypeNotFound) {
		t.Fatalf("rename missing = %v", err)
	}
	if err := service.DeleteSportType(ctx, "UNUSED"); !errors.Is(err, ErrSportTypeNotFound) {
		t.Fatalf("delete missing = %v", err)
	}

	for _, reference := range sportReferences {
		t.Run(reference.name, func(t *testing.T) {
			id := reference.table
			if _, err := service.CreateSportType(ctx, SportType{ID: id, Title: "Sport"}); err != nil {
				t.Fatal(err)
			}
			if _, err := p.SQL.Exec(reference.insert, id); err != nil {
				t.Fatal(err)
			}
			if reference.table == "group_heads" {
				execSportSQL(t, p, `INSERT INTO group_lines(group_id,team_id) VALUES('group_heads','A')`)
			}
			if err := service.DeleteSportType(ctx, id); !errors.Is(err, ErrSportTypeInUse) {
				t.Fatalf("referenced delete = %v", err)
			}
			var count int
			// Table names come from the static test fixture above.
			if err := p.SQL.QueryRow("SELECT count(*) FROM "+reference.table+" WHERE type_id = $1", id).Scan(&count); err != nil || count != 1 {
				t.Fatalf("dependent record count = %d; err=%v", count, err)
			}
			if _, err := service.GetSportType(ctx, id); err != nil {
				t.Fatalf("referenced catalogue entry removed: %v", err)
			}
		})
	}
	var groupLines int
	if err := p.SQL.QueryRow(`SELECT count(*) FROM group_lines`).Scan(&groupLines); err != nil || groupLines != 1 {
		t.Fatalf("group memberships removed: %d %v", groupLines, err)
	}
}

func TestSportTypeFreshSchemaRestrictsReferences(t *testing.T) {
	p := sportTypePostgres(t)
	if err := p.Migrate(); err != nil {
		t.Fatalf("repeat migration must be a no-op: %v", err)
	}
	for _, reference := range sportReferences {
		var rule string
		if err := p.SQL.QueryRow(`SELECT confdeltype FROM pg_constraint WHERE conname=$1`, reference.table+"_type_id_fkey").Scan(&rule); err != nil || rule != "r" {
			t.Fatalf("%s deletion rule = %s; err=%v", reference.table, rule, err)
		}
	}
}

func waitSportTypeBlock(t *testing.T, p *testutil.Postgres, blocker, waiter int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var blocked bool
		if err := p.SQL.QueryRow(`SELECT $1 = ANY(pg_blocking_pids($2))`, blocker, waiter).Scan(&blocked); err != nil {
			t.Fatal(err)
		}
		if blocked {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("concurrent statement did not wait on the foreign-key lock")
}

func TestSportTypeConcurrentReferenceAndDeletion(t *testing.T) {
	p := sportTypePostgres(t)
	execSportSQL(t, p, `INSERT INTO colors(id,title) VALUES('A','A')`)
	for _, reference := range sportReferences {
		for _, referenceFirst := range []bool{true, false} {
			name := reference.name + "/delete first"
			if referenceFirst {
				name = reference.name + "/reference first"
			}
			t.Run(name, func(t *testing.T) {
				id := reference.table + "_delete"
				if referenceFirst {
					id = reference.table + "_reference"
				}
				if _, err := p.SQL.Exec(`INSERT INTO sport_types(id,title) VALUES($1,'Sport')`, id); err != nil {
					t.Fatal(err)
				}
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				refTx, err := p.SQL.BeginTx(ctx, nil)
				if err != nil {
					t.Fatal(err)
				}
				defer func() {
					if err := refTx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
						t.Error(err)
					}
				}()
				deleteTx := p.DB.WithContext(ctx).Begin()
				if deleteTx.Error != nil {
					t.Fatal(deleteTx.Error)
				}
				defer func() {
					if err := deleteTx.Rollback().Error; err != nil && !errors.Is(err, sql.ErrTxDone) && !errors.Is(err, gorm.ErrInvalidTransaction) {
						t.Error(err)
					}
				}()
				var refPID, deletePID int
				if err := refTx.QueryRowContext(ctx, `SELECT pg_backend_pid()`).Scan(&refPID); err != nil {
					t.Fatal(err)
				}
				if err := deleteTx.Raw(`SELECT pg_backend_pid()`).Scan(&deletePID).Error; err != nil {
					t.Fatal(err)
				}
				repo := NewGORMRepository(deleteTx)
				done := make(chan error, 1)
				if referenceFirst {
					if _, err := refTx.ExecContext(ctx, reference.insert, id); err != nil {
						t.Fatal(err)
					}
					go func() { done <- repo.DeleteSportType(ctx, id) }()
					waitSportTypeBlock(t, p, refPID, deletePID)
					if err := refTx.Commit(); err != nil {
						t.Fatal(err)
					}
					if err := <-done; !errors.Is(err, ErrSportTypeInUse) {
						t.Fatalf("concurrent deletion = %v", err)
					}
				} else {
					if err := repo.DeleteSportType(ctx, id); err != nil {
						t.Fatal(err)
					}
					go func() {
						_, err := refTx.ExecContext(ctx, reference.insert, id)
						done <- err
					}()
					waitSportTypeBlock(t, p, deletePID, refPID)
					if err := deleteTx.Commit().Error; err != nil {
						t.Fatal(err)
					}
					var pgErr *pgconn.PgError
					if err := <-done; !errors.As(err, &pgErr) || pgErr.Code != "23503" {
						t.Fatalf("concurrent reference creation = %v", err)
					}
				}
			})
		}
		var dangling int
		if err := p.SQL.QueryRow("SELECT count(*) FROM " + reference.table + " r LEFT JOIN sport_types s ON s.id=r.type_id WHERE s.id IS NULL").Scan(&dangling); err != nil || dangling != 0 {
			t.Fatalf("dangling %s references = %d; err=%v", reference.name, dangling, err)
		}
	}
}
