package bill

import (
	"errors"
	"testing"

	"gorm.io/gorm"
)

func TestBillLookupPreservesFeatureAndDriverCause(t *testing.T) {
	err := billLookupError(gorm.ErrRecordNotFound)
	if !errors.Is(err, ErrNotFound) || !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("lookup cause was lost: %v", err)
	}
	cause := errors.New("database unavailable")
	if !errors.Is(billLookupError(cause), cause) {
		t.Fatal("infrastructure error was converted to a missing bill")
	}
}
