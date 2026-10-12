package user

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"gorm.io/gorm"
)

func TestMapUserLookupErrorOnlyMapsMissingRecords(t *testing.T) {
	databaseFailure := fmt.Errorf("query users: %w", context.DeadlineExceeded)
	tests := []struct {
		name         string
		err          error
		wantNotFound bool
		wantCause    error
	}{
		{
			name:         "record not found",
			err:          fmt.Errorf("query user: %w", gorm.ErrRecordNotFound),
			wantNotFound: true,
			wantCause:    gorm.ErrRecordNotFound,
		},
		{
			name:         "database timeout",
			err:          databaseFailure,
			wantNotFound: false,
			wantCause:    context.DeadlineExceeded,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := mapUserLookupError(test.err)
			if errors.Is(got, ErrUserNotFound) != test.wantNotFound {
				t.Fatalf("errors.Is(err, ErrUserNotFound) = %v; want %v", errors.Is(got, ErrUserNotFound), test.wantNotFound)
			}
			if !errors.Is(got, test.wantCause) {
				t.Fatalf("mapped error %v does not preserve cause %v", got, test.wantCause)
			}
		})
	}
}
