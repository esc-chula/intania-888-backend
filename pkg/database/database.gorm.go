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
func NewGORMDatabase(cfg config.Config) *gorm.DB {
	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%d sslmode=%s TimeZone=%s",
		cfg.GetDB().Host,
		cfg.GetDB().User,
		cfg.GetDB().Password,
		cfg.GetDB().Name,
		cfg.GetDB().Port,
		cfg.GetDB().SSLMode,
		cfg.GetDB().Timezone,
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

	return db
}
