package logger

import (
	"go.uber.org/zap"

	"github.com/esc-chula/intania-888-backend/pkg/config"
)

const (
	// DEV selects Zap development logging.
	DEV = "development"
	// PROD selects Zap production logging.
	PROD = "production"
)

// NewLogger constructs the Zap logger for the configured server environment.
// It returns nil for an unsupported environment and panics if Zap initialization fails;
// startup validation must reject unsupported environments before this constructor.
func NewLogger(cfg config.Config) *zap.Logger {
	return newLoggerFactory(cfg.GetServer().Env)
}

func newLoggerFactory(env string) *zap.Logger {
	switch env {
	case DEV:
		return zap.Must(zap.NewDevelopment())
	case PROD:
		return zap.Must(zap.NewProduction())
	default:
		return nil
	}
}
