package match

import (
	"math"
	"testing"
)

func TestRateForLargeCounts(t *testing.T) {
	rate, err := rateFor(math.MaxInt64, math.MaxInt64, true)

	if err != nil {
		t.Fatal(err)
	}

	if rate.MicroUnits() != 2_000_000 {
		t.Fatalf("rate = %d; want 2000000", rate.MicroUnits())
	}
}

func TestRateForRejectsInvalidAndOverflowingCounts(t *testing.T) {
	if _, err := rateFor(-1, 0, true); err == nil {
		t.Fatal("negative count must be rejected")
	}

	if _, err := rateFor(math.MaxInt64, 0, false); err == nil {
		t.Fatal("unrepresentable rate must be rejected")
	}
}
