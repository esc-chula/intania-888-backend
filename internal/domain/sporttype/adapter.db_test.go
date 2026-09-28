package sporttype

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

func TestSportTypeStorageErrorTranslation(t *testing.T) {
	unknown := errors.New("storage failed")
	tests := []struct{ input, want error }{
		{nil, nil},
		{gorm.ErrRecordNotFound, ErrSportTypeNotFound},
		{fmt.Errorf("wrapped: %w", gorm.ErrRecordNotFound), ErrSportTypeNotFound},
		{&pgconn.PgError{Code: "23505"}, ErrSportTypeConflict},
		{fmt.Errorf("wrapped: %w", &pgconn.PgError{Code: "23503"}), ErrSportTypeInUse},
		{&pgconn.PgError{Code: "23001"}, ErrSportTypeInUse},
		{unknown, unknown},
	}
	for _, test := range tests {
		if err := translateStorageError(test.input); !errors.Is(err, test.want) {
			t.Fatalf("translate(%v) = %v; want %v", test.input, err, test.want)
		}
	}
}
