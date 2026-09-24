//go:build integration

package database_test

import (
	"os"
	"testing"

	"github.com/esc-chula/intania-888-backend/internal/testutil"
)

func TestMigrationsUpDownUpAndConstraints(t *testing.T) {
	dsn := os.Getenv("INTANIA888_TEST_DATABASE_URL")
	p, err := testutil.OpenPostgres(dsn)
	if err != nil {
		if err == testutil.ErrMissingTestDatabaseURL {
			t.Skip(err)
		}

		t.Fatal(err)
	}

	defer func() {
		if closeErr := p.Close(); closeErr != nil {
			t.Errorf("close migration database: %v", closeErr)
		}
	}()

	if err := p.ResetAndMigrate(); err != nil {
		t.Fatal(err)
	}

	if err := p.Migrate(); err != nil {
		t.Fatalf("repeated up must be a no-op: %v", err)
	}

	assertSchemaTypes(t, p)
	assertMigrationConstraints(t, p)

	if err := p.Reset(); err != nil {
		t.Fatal(err)
	}

	if err := p.Migrate(); err != nil {
		t.Fatal(err)
	}
}

func assertSchemaTypes(t *testing.T, p *testutil.Postgres) {
	t.Helper()

	columns := []struct {
		table  string
		column string
	}{
		{table: "users", column: "remaining_coin"},
		{table: "bill_heads", column: "total"},
		{table: "bill_heads", column: "payout"},
		{table: "bill_lines", column: "rate"},
		{table: "daily_rewards", column: "reward"},
		{table: "mine_games", column: "bet_amount"},
		{table: "mine_games", column: "current_payout"},
		{table: "mine_games", column: "multiplier"},
		{table: "mine_game_histories", column: "multiplier"},
		{table: "mine_game_histories", column: "payout_at_hit"},
	}

	for _, column := range columns {
		var dataType string

		err := p.SQL.QueryRow(`
SELECT data_type
FROM information_schema.columns
WHERE table_schema = 'public' AND table_name = $1 AND column_name = $2`, column.table, column.column).Scan(&dataType)
		if err != nil {
			t.Fatalf("inspect %s.%s: %v", column.table, column.column, err)
		}

		if dataType != "bigint" {
			t.Fatalf("%s.%s type = %s; want bigint", column.table, column.column, dataType)
		}
	}

	var decimalColumns int
	if err := p.SQL.QueryRow(`
SELECT count(*)
FROM information_schema.columns
WHERE table_schema = 'public'
  AND data_type IN ('numeric', 'double precision', 'real')`).Scan(&decimalColumns); err != nil {
		t.Fatal(err)
	}

	if decimalColumns != 0 {
		t.Fatalf("found %d legacy decimal columns", decimalColumns)
	}
}

func assertMigrationConstraints(t *testing.T, p *testutil.Postgres) {
	t.Helper()

	var roleCount int
	if err := p.SQL.QueryRow(`SELECT count(*) FROM roles`).Scan(&roleCount); err != nil {
		t.Fatal(err)
	}

	if roleCount != 2 {
		t.Fatalf("role count = %d; want 2", roleCount)
	}

	var catalogueCount int
	if err := p.SQL.QueryRow(`
SELECT (SELECT count(*) FROM colors) + (SELECT count(*) FROM matches) + (SELECT count(*) FROM sport_types)`).Scan(&catalogueCount); err != nil {
		t.Fatal(err)
	}

	if catalogueCount != 0 {
		t.Fatalf("migration imported %d catalogue or schedule rows", catalogueCount)
	}

	setup := []string{
		`INSERT INTO colors(id,title) VALUES('A','A'),('B','B')`,
		`INSERT INTO sport_types(id,title) VALUES('S','S')`,
		`INSERT INTO users(id,email,name,role_id) VALUES('U','u@example.test','u','USER')`,
		`INSERT INTO matches(id,teama_id,teamb_id,type_id,start_time,end_time) VALUES('M','A','B','S',now(),now())`,
	}

	for _, statement := range setup {
		if _, err := p.SQL.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}

	statements := []struct {
		name string
		sql  string
	}{
		{
			name: "negative balance",
			sql:  `INSERT INTO users(id,email,name,role_id,remaining_coin) VALUES('negative','negative@example.test','negative','USER',-1)`,
		},
		{
			name: "negative bill total",
			sql:  `INSERT INTO bill_heads(id,total,user_id) VALUES('negative-bill',-1,'U')`,
		},
		{
			name: "negative bill payout",
			sql:  `INSERT INTO bill_heads(id,total,user_id,payout) VALUES('negative-payout',100,'U',-1)`,
		},
		{
			name: "negative daily reward",
			sql:  `INSERT INTO daily_rewards(date,reward) VALUES('negative',-1)`,
		},
		{
			name: "negative mine stake",
			sql:  `INSERT INTO mine_games(id,user_id,bet_amount,risk_level,status,current_payout,multiplier,grid_data) VALUES('negative-stake','U',-1,'low','active',0,1000000,'[]')`,
		},
		{
			name: "negative mine payout",
			sql:  `INSERT INTO mine_games(id,user_id,bet_amount,risk_level,status,current_payout,multiplier,grid_data) VALUES('negative-payout-game','U',0,'low','active',-1,1000000,'[]')`,
		},
		{
			name: "negative mine multiplier",
			sql:  `INSERT INTO mine_games(id,user_id,bet_amount,risk_level,status,current_payout,multiplier,grid_data) VALUES('negative-multiplier','U',0,'low','active',0,-1,'[]')`,
		},
	}

	for _, statement := range statements {
		if _, err := p.SQL.Exec(statement.sql); err == nil {
			t.Fatalf("%s constraint did not reject row", statement.name)
		}
	}

	if _, err := p.SQL.Exec(`INSERT INTO bill_heads(id,total,user_id) VALUES('B','100','U')`); err != nil {
		t.Fatal(err)
	}

	if _, err := p.SQL.Exec(`INSERT INTO bill_lines(bill_id,match_id,rate,betting_on) VALUES('B','M',-1,'A')`); err == nil {
		t.Fatal("negative bill rate constraint did not reject row")
	}

	if _, err := p.SQL.Exec(`INSERT INTO mine_games(id,user_id,bet_amount,risk_level,status,current_payout,multiplier,grid_data) VALUES('G1','U',100,'low','active',100,1000000,'[]')`); err != nil {
		t.Fatal(err)
	}

	if _, err := p.SQL.Exec(`INSERT INTO mine_game_histories(id,game_id,tile_index,tile_type,multiplier,payout_at_hit) VALUES('H1','G1',0,'safe',1000000,-1)`); err == nil {
		t.Fatal("negative history payout constraint did not reject row")
	}

	if _, err := p.SQL.Exec(`INSERT INTO mine_games(id,user_id,bet_amount,risk_level,status,current_payout,multiplier,grid_data) VALUES('G2','U',100,'low','active',100,1000000,'[]')`); err == nil {
		t.Fatal("active-game partial unique index did not reject second game")
	}

	if _, err := p.SQL.Exec(`INSERT INTO matches(id,teama_id,teamb_id,winner_id,type_id,start_time,end_time) VALUES('invalid-winner','A','B','missing','S',now(),now())`); err == nil {
		t.Fatal("winner membership constraint did not reject row")
	}
}
