package testutil

import (
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ErrMissingTestDatabaseURL allows optional integration suites to skip when
// their disposable PostgreSQL connection has not been configured.
var ErrMissingTestDatabaseURL = errors.New("INTANIA888_TEST_DATABASE_URL is not set")

// Postgres exposes ORM and SQL connections to the same disposable test database.
// Reset and migration helpers change its schema; callers must isolate each suite.
type Postgres struct {
	DB  *gorm.DB
	SQL *sql.DB
}

// OpenPostgres opens and pings a database whose URL name contains "test".
// An empty URL returns ErrMissingTestDatabaseURL for optional suites; setting
// INTANIA888_REQUIRE_INTEGRATION=1 instead returns a failure that must not be skipped.
// A failed ping closes the connection and joins any cleanup error with the cause.
func OpenPostgres(dsn string) (*Postgres, error) {
	if strings.TrimSpace(dsn) == "" {
		if os.Getenv("INTANIA888_REQUIRE_INTEGRATION") == "1" {
			return nil, errors.New("required integration configuration missing: INTANIA888_TEST_DATABASE_URL")
		}
		return nil, ErrMissingTestDatabaseURL
	}

	u, err := url.Parse(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse test database URL: %w", err)
	}

	databaseName := strings.TrimSpace(strings.TrimPrefix(u.Path, "/"))
	if !strings.Contains(strings.ToLower(databaseName), "test") {
		return nil, fmt.Errorf("refusing database %q; test database name must contain test", databaseName)
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("open test database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get test database connection: %w", err)
	}

	sqlDB.SetMaxOpenConns(32)
	sqlDB.SetMaxIdleConns(32)

	if err := sqlDB.Ping(); err != nil {
		pingErr := fmt.Errorf("ping test database: %w", err)
		if closeErr := sqlDB.Close(); closeErr != nil {
			return nil, errors.Join(pingErr, fmt.Errorf("close failed test database: %w", closeErr))
		}
		return nil, pingErr
	}

	return &Postgres{DB: db, SQL: sqlDB}, nil
}

// Close releases the SQL pool and is safe for a nil Postgres or missing pool.
func (p *Postgres) Close() error {
	if p == nil || p.SQL == nil {
		return nil
	}

	return p.SQL.Close()
}

// Reset runs Goose down migrations against the disposable test database.
// It destroys migrated tables and their data; suites must not share this database
// while reset or migration helpers are running.
func (p *Postgres) Reset() error {
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}

	if _, err := goose.EnsureDBVersion(p.SQL); err != nil {
		return err
	}

	return goose.Reset(p.SQL, migrationsPath())
}

// Migrate applies all pending repository migrations to the test database.
func (p *Postgres) Migrate() error {
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}

	return goose.Up(p.SQL, migrationsPath())
}

// ResetAndMigrate rebuilds the disposable database from repository migrations.
// It returns the reset error without attempting a new migration when reset fails.
func (p *Postgres) ResetAndMigrate() error {
	if err := p.Reset(); err != nil {
		return err
	}

	return p.Migrate()
}

// InstallFailureTrigger makes a table operation fail before each affected row.
// Operation is a trusted SQL event clause supplied by a test, such as INSERT.
// The returned cleanup removes the trigger and function; installation failures
// remove the partial function and join any cleanup error with the original cause.
func (p *Postgres) InstallFailureTrigger(table, operation string) (func() error, error) {
	identifier := strings.ReplaceAll(uuid.NewString(), "-", "")
	functionName := "test_fail_" + identifier
	triggerName := "test_fail_trigger_" + identifier

	functionSQL := fmt.Sprintf(`
CREATE FUNCTION %s() RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'injected integration failure';
END;
$$`, quoteIdentifier(functionName))

	if _, err := p.SQL.Exec(functionSQL); err != nil {
		return nil, err
	}

	triggerSQL := fmt.Sprintf(
		"CREATE TRIGGER %s BEFORE %s ON %s FOR EACH ROW EXECUTE FUNCTION %s()",
		quoteIdentifier(triggerName),
		operation,
		quoteIdentifier(table),
		quoteIdentifier(functionName),
	)

	if _, err := p.SQL.Exec(triggerSQL); err != nil {
		_, cleanupErr := p.SQL.Exec(fmt.Sprintf("DROP FUNCTION %s()", quoteIdentifier(functionName)))
		if cleanupErr != nil {
			return nil, errors.Join(err, fmt.Errorf("remove partial failure function: %w", cleanupErr))
		}
		return nil, err
	}

	return func() error {
		if _, err := p.SQL.Exec(fmt.Sprintf("DROP TRIGGER IF EXISTS %s ON %s", quoteIdentifier(triggerName), quoteIdentifier(table))); err != nil {
			return err
		}

		_, err := p.SQL.Exec(fmt.Sprintf("DROP FUNCTION IF EXISTS %s()", quoteIdentifier(functionName)))

		return err
	}, nil
}

func migrationsPath() string {
	_, file, _, _ := runtime.Caller(0)

	return filepath.Join(filepath.Dir(file), "..", "..", "migrations")
}

func quoteIdentifier(identifier string) string {
	return `"` + strings.ReplaceAll(identifier, `"`, `""`) + `"`
}
