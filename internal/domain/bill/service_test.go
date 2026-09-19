package bill

import (
	"math"
	"testing"
)

func TestSeededRate(t *testing.T) {
	tests := []struct {
		a, b int64
		forA bool
		want int64
	}{
		{a: 0, b: 0, forA: true, want: 2000000},
		{a: 0, b: 0, forA: false, want: 2000000},
		{a: 3, b: 1, forA: true, want: 1500000},
		{a: 3, b: 1, forA: false, want: 3000000},
		{a: 1, b: 2, forA: true, want: 2500000},
		{a: math.MaxInt64, b: math.MaxInt64, forA: true, want: 2000000},
	}

	for _, tc := range tests {
		got, err := seededRate(tc.a, tc.b, tc.forA)

		if err != nil || got.MicroUnits() != tc.want {
			t.Fatalf("seededRate(%d,%d,%v)=%d,%v; want %d", tc.a, tc.b, tc.forA, got.MicroUnits(), err, tc.want)
		}
	}
}

func TestSeededRateRejectsInvalidAndOverflowingCounts(t *testing.T) {
	if _, err := seededRate(-1, 0, true); err == nil {
		t.Fatal("negative count must be rejected")
	}

	if _, err := seededRate(math.MaxInt64, 0, false); err == nil {
		t.Fatal("unrepresentable rate must be rejected")
	}
}
