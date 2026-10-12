// Package betting provides the fixed-point odds shared by bill creation and match views.
package betting

import (
	"math/big"

	"github.com/esc-chula/intania-888-backend/internal/value"
)

// SeededRate computes (a+b+2)/(selected+1), rounded half up to six decimal places.
// Pool amounts are minor currency units; negative inputs and overflowing rates fail.
func SeededRate(a, b int64, forA bool) (value.Rate, error) {
	if a < 0 || b < 0 {
		return value.Rate{}, value.ErrInvalidRate
	}

	den := new(big.Int).Add(big.NewInt(a), big.NewInt(1))

	if !forA {
		den = new(big.Int).Add(big.NewInt(b), big.NewInt(1))
	}

	total := new(big.Int).Add(big.NewInt(a), big.NewInt(b))
	total.Add(total, big.NewInt(2))

	n := new(big.Int).Mul(total, big.NewInt(1_000_000))
	q, r := new(big.Int), new(big.Int)
	q.QuoRem(n, den, r)

	if new(big.Int).Lsh(r, 1).Cmp(den) >= 0 {
		q.Add(q, big.NewInt(1))
	}

	if !q.IsInt64() {
		return value.Rate{}, value.ErrOverflow
	}

	return value.NewRateFromMicro(q.Int64())
}
