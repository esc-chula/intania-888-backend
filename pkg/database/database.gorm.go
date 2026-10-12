package database

import (
	"fmt"
	"log"
	"os"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/esc-chula/intania-888-backend/pkg/config"
)

// NewGORMDatabase opens PostgreSQL using the configured credentials and SQL logger.
// It preserves the existing startup contract and panics if the connection cannot open.
func NewGORMDatabase(cfg config.DatabaseConfig) *gorm.DB {
	settings := cfg.GetDB()
	if settings.MaxOpenConns <= 0 || settings.MaxIdleConns < 0 ||
		settings.MaxIdleConns > settings.MaxOpenConns ||
		settings.ConnMaxLifetimeSeconds < 0 || settings.ConnMaxIdleTimeSeconds < 0 {
		panic("invalid database connection pool settings")
	}

	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%d sslmode=%s TimeZone=%s",
		settings.Host,
		settings.User,
		settings.Password,
		settings.Name,
		settings.Port,
		settings.SSLMode,
		settings.Timezone,
	)

	logger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags), // io writer
		logger.Config{
			SlowThreshold: time.Second,  // Slow SQL threshold
			LogLevel:      logger.Error, // Log level
			Colorful:      true,         // Disable color
		},
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger:                                   logger,
		DisableForeignKeyConstraintWhenMigrating: false,
	})
	if err != nil {
		panic("failed to connect database")
	}

	sqlDB, err := db.DB()
	if err != nil {
		panic("failed to configure database connection pool")
	}

	sqlDB.SetMaxOpenConns(settings.MaxOpenConns)
	sqlDB.SetMaxIdleConns(settings.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(time.Duration(settings.ConnMaxLifetimeSeconds) * time.Second)
	sqlDB.SetConnMaxIdleTime(time.Duration(settings.ConnMaxIdleTimeSeconds) * time.Second)

	return db
}
