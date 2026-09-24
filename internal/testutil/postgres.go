package testutil

import (
	"database/sql"
	"errors"
	"fmt"
	"net/url"
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

var ErrMissingTestDatabaseURL = errors.New("INTANIA888_TEST_DATABASE_URL is not set")

type Postgres struct {
	DB  *gorm.DB
	SQL *sql.DB
}

func OpenPostgres(dsn string) (*Postgres, error) {
	if strings.TrimSpace(dsn) == "" {
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
		_ = sqlDB.Close()
		return nil, fmt.Errorf("ping test database: %w", err)
	}

	return &Postgres{DB: db, SQL: sqlDB}, nil
}

func (p *Postgres) Close() error {
	if p == nil || p.SQL == nil {
		return nil
	}

	return p.SQL.Close()
}

func (p *Postgres) Reset() error {
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}

	if _, err := goose.EnsureDBVersion(p.SQL); err != nil {
		return err
	}

	return goose.Reset(p.SQL, migrationsPath())
}

func (p *Postgres) Migrate() error {
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}

	return goose.Up(p.SQL, migrationsPath())
}

func (p *Postgres) ResetAndMigrate() error {
	if err := p.Reset(); err != nil {
		return err
	}

	return p.Migrate()
}

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
		_, _ = p.SQL.Exec(fmt.Sprintf("DROP FUNCTION %s()", quoteIdentifier(functionName)))
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
