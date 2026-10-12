package policy

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

func TestStorageErrorsTranslateToPolicyContracts(t *testing.T) {
	unexpected := errors.New("unexpected storage failure")
	tests := []struct {
		name          string
		storage, want error
	}{
		{"missing", gorm.ErrRecordNotFound, ErrPolicyNotFound},
		{"duplicate identity", &pgconn.PgError{Code: "23505"}, ErrPolicyConflict},
		{"unexpected preserved", unexpected, unexpected},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := translateStorageError(test.storage); !errors.Is(err, test.want) {
				t.Fatalf("storage translation=%v, want %v", err, test.want)
			}
		})
	}
}
