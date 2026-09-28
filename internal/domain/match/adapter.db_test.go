package match

import (
	"errors"
	"testing"

	"gorm.io/gorm"
)

func TestMatchLookupPreservesFeatureAndDriverCause(t *testing.T) {
	err := matchLookupError(gorm.ErrRecordNotFound)
	if !errors.Is(err, ErrNotFound) || !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("lookup cause was lost: %v", err)
	}
	cause := errors.New("database unavailable")
	if !errors.Is(matchLookupError(cause), cause) {
		t.Fatal("infrastructure error was converted to a missing match")
	}
}
