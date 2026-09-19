package database_test

import (
	"database/sql"
	"net/url"
	"os"
	"strings"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

func TestMigrationsUpDownUpAndConstraints(t *testing.T) {
	dsn := os.Getenv("INTANIA888_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("INTANIA888_TEST_DATABASE_URL is not set")
	}

	u, err := url.Parse(dsn)
	if err != nil || !strings.Contains(strings.ToLower(strings.TrimPrefix(u.Path, "/")), "test") {
		t.Fatal("refusing migration test: database name must contain 'test'")
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}

	defer db.Close()

	if err = db.Ping(); err != nil {
		t.Fatal(err)
	}

	if err = goose.SetDialect("postgres"); err != nil {
		t.Fatal(err)
	}

	const dir = "../../migrations"

	if err = goose.Reset(db, dir); err != nil {
		t.Fatal(err)
	}

	if err = goose.Up(db, dir); err != nil {
		t.Fatal(err)
	}

	if err = goose.Up(db, dir); err != nil {
		t.Fatalf("repeated up must be a no-op: %v", err)
	}

	if _, err = db.Exec(`INSERT INTO users(id,email,name,role_id,remaining_coin) VALUES('negative','n@example.test','n','USER',-1)`); err == nil {
		t.Fatal("negative balance constraint did not reject row")
	}

	setup := []string{
		`INSERT INTO colors(id,title) VALUES('A','A'),('B','B')`,
		`INSERT INTO sport_types(id,title) VALUES('S','S')`,
		`INSERT INTO matches(id,teama_id,teamb_id,type_id,start_time,end_time) VALUES('M','A','B','S',now(),now())`,
		`INSERT INTO users(id,email,name,role_id) VALUES('U','u@example.test','u','USER')`,
		`INSERT INTO mine_games(id,user_id,bet_amount,risk_level,status,current_payout,multiplier,grid_data) VALUES('G1','U',100,'low','active',100,1000000,'[]')`,
	}

	for _, statement := range setup {
		if _, err = db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}

	if _, err = db.Exec(`INSERT INTO mine_games(id,user_id,bet_amount,risk_level,status,current_payout,multiplier,grid_data) VALUES('G2','U',100,'low','active',100,1000000,'[]')`); err == nil {
		t.Fatal("active-game partial unique index did not reject second game")
	}

	if err = goose.Down(db, dir); err != nil {
		t.Fatal(err)
	}

	if err = goose.Up(db, dir); err != nil {
		t.Fatal(err)
	}
}
