package value

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
)

const rateScale int64 = 1_000_000

var (
	// ErrInvalidMoney indicates invalid money syntax or a negative Money result.
	ErrInvalidMoney = errors.New("invalid money value")
	// ErrInvalidRate indicates invalid rate syntax or a negative rate.
	ErrInvalidRate = errors.New("invalid rate value")
	// ErrOverflow indicates checked arithmetic exceeds the int64 result range.
	ErrOverflow = errors.New("fixed-point arithmetic overflow")
)

// Money is a non-negative amount stored in minor (hundredth) units.
// It deliberately has no database interfaces; persistence rows store int64s.
// The zero value is a valid zero balance.
type Money struct {
	minor int64
}

// NewMoneyFromMinor constructs a non-negative amount from hundredth units.
func NewMoneyFromMinor(minor int64) (Money, error) {
	if minor < 0 {
		return Money{}, ErrInvalidMoney
	}

	return Money{minor: minor}, nil
}

// MustMoneyFromMinor constructs Money and panics for a negative amount.
// Use it only when a constant or previously checked invariant guarantees validity.
func MustMoneyFromMinor(minor int64) Money {
	m, err := NewMoneyFromMinor(minor)

	if err != nil {
		panic(err)
	}

	return m
}

// ParseMoney accepts a non-negative decimal with at most two fractional digits.
// Whitespace, signs, exponent notation, and amounts outside int64 minor units
// return ErrInvalidMoney; shorter fractions are padded without rounding.
func ParseMoney(s string) (Money, error) {
	v, err := parseFixed(s, 2, false)

	if err != nil {
		return Money{}, ErrInvalidMoney
	}

	return NewMoneyFromMinor(v)
}

// MinorUnits returns the exact number of hundredth units.
func (m Money) MinorUnits() int64 { return m.minor }

// IsZero reports whether the amount is zero.
func (m Money) IsZero() bool { return m.minor == 0 }

// String returns the canonical decimal amount with exactly two fractional digits.
func (m Money) String() string { return formatFixed(m.minor, 2) }

// Compare returns -1, 0, or 1 when m is less than, equal to, or greater than other.
func (m Money) Compare(other Money) int {
	if m.minor < other.minor {
		return -1
	}

	if m.minor > other.minor {
		return 1
	}

	return 0
}

// Greater reports whether m exceeds other.
func (m Money) Greater(other Money) bool { return m.minor > other.minor }

// Lesser reports whether m is below other.
func (m Money) Lesser(other Money) bool { return m.minor < other.minor }

// Add returns the exact sum, or ErrOverflow if it exceeds int64 minor units.
func (m Money) Add(other Money) (Money, error) {
	if other.minor > math.MaxInt64-m.minor {
		return Money{}, ErrOverflow
	}

	return Money{minor: m.minor + other.minor}, nil
}

// Sub returns the exact difference, or ErrInvalidMoney if it would be negative.
func (m Money) Sub(other Money) (Money, error) {
	if other.minor > m.minor {
		return Money{}, ErrInvalidMoney
	}

	return Money{minor: m.minor - other.minor}, nil
}

// Mul applies a fixed-point rate and rounds half up to the nearest minor unit.
// Intermediate arithmetic is unbounded; an unrepresentable result returns ErrOverflow.
func (m Money) Mul(rate Rate) (Money, error) {
	return moneyFromBigRounded(
		new(big.Int).Mul(big.NewInt(m.minor), big.NewInt(rate.micro)),
		big.NewInt(rateScale),
	)
}

// MarshalJSON encodes Money as a quoted decimal with two fractional digits.
func (m Money) MarshalJSON() ([]byte, error) { return json.Marshal(m.String()) }

// UnmarshalJSON accepts a decimal JSON string using ParseMoney.
// Numeric tokens and null are rejected, and failures leave the receiver unchanged.
func (m *Money) UnmarshalJSON(data []byte) error {
	if m == nil || len(data) == 0 || data[0] != '"' {
		return ErrInvalidMoney
	}

	var raw string

	if err := json.Unmarshal(data, &raw); err != nil {
		return ErrInvalidMoney
	}

	parsed, err := ParseMoney(raw)

	if err != nil {
		return err
	}

	*m = parsed

	return nil
}

// SignedMoney is used for derived deltas that may be negative.
// It stores hundredth units across the full int64 range; the zero value is valid.
type SignedMoney struct {
	minor int64
}

// NewSignedMoneyFromMinor constructs a signed delta from hundredth units.
func NewSignedMoneyFromMinor(minor int64) SignedMoney { return SignedMoney{minor: minor} }

// ParseSignedMoney accepts a decimal with an optional minus and at most two
// fractional digits. Invalid syntax or amounts outside int64 minor units return
// ErrInvalidMoney; whitespace, plus signs, and exponent notation are rejected.
func ParseSignedMoney(s string) (SignedMoney, error) {
	v, err := parseFixed(s, 2, true)

	if err != nil {
		return SignedMoney{}, ErrInvalidMoney
	}

	return SignedMoney{minor: v}, nil
}

// MinorUnits returns the exact signed number of hundredth units.
func (m SignedMoney) MinorUnits() int64 { return m.minor }

// String returns the signed decimal with exactly two fractional digits.
func (m SignedMoney) String() string { return formatFixed(m.minor, 2) }

// MarshalJSON encodes the signed amount as a quoted decimal string.
func (m SignedMoney) MarshalJSON() ([]byte, error) { return json.Marshal(m.String()) }

// Add returns the exact signed sum, or ErrOverflow outside the int64 range.
func (m SignedMoney) Add(other SignedMoney) (SignedMoney, error) {
	positiveOverflow := other.minor > 0 && m.minor > math.MaxInt64-other.minor
	negativeOverflow := other.minor < 0 && m.minor < math.MinInt64-other.minor

	if positiveOverflow || negativeOverflow {
		return SignedMoney{}, ErrOverflow
	}

	return SignedMoney{minor: m.minor + other.minor}, nil
}

// Sub returns the exact signed difference, or ErrOverflow outside the int64 range.
func (m SignedMoney) Sub(other SignedMoney) (SignedMoney, error) {
	result := new(big.Int).Sub(big.NewInt(m.minor), big.NewInt(other.minor))

	if !result.IsInt64() {
		return SignedMoney{}, ErrOverflow
	}

	return SignedMoney{minor: result.Int64()}, nil
}

// UnmarshalJSON accepts a decimal JSON string using ParseSignedMoney.
// Numeric tokens and null are rejected, and failures leave the receiver unchanged.
func (m *SignedMoney) UnmarshalJSON(data []byte) error {
	if m == nil || len(data) == 0 || data[0] != '"' {
		return ErrInvalidMoney
	}

	var raw string

	if err := json.Unmarshal(data, &raw); err != nil {
		return ErrInvalidMoney
	}

	v, err := ParseSignedMoney(raw)

	if err != nil {
		return err
	}

	*m = v

	return nil
}

// Rate stores a non-negative value with six decimal places.
// One is 1,000,000 micro units. The zero value is valid, and rates may exceed one.
type Rate struct {
	micro int64
}

// NewRateFromMicro constructs a non-negative rate from millionth units.
func NewRateFromMicro(micro int64) (Rate, error) {
	if micro < 0 {
		return Rate{}, ErrInvalidRate
	}

	return Rate{micro: micro}, nil
}

// MustRateFromMicro constructs Rate and panics for a negative value.
// Use it only for constants or values whose validity has already been checked.
func MustRateFromMicro(micro int64) Rate {
	r, err := NewRateFromMicro(micro)

	if err != nil {
		panic(err)
	}

	return r
}

// ParseRate accepts a non-negative decimal with at most six fractional digits.
// Invalid syntax or values outside int64 millionth units return ErrInvalidRate;
// shorter fractions are padded without rounding.
func ParseRate(s string) (Rate, error) {
	v, err := parseFixed(s, 6, false)

	if err != nil {
		return Rate{}, ErrInvalidRate
	}

	return NewRateFromMicro(v)
}

// MicroUnits returns the exact number of millionth units.
func (r Rate) MicroUnits() int64 { return r.micro }

// String returns the canonical rate with exactly six fractional digits.
func (r Rate) String() string { return formatFixed(r.micro, 6) }

// MarshalJSON returns the rate as a decimal JSON number.
// Rate remains a JSON number for compatibility, while calculations remain fixed-point.
func (r Rate) MarshalJSON() ([]byte, error) { return []byte(r.String()), nil }

// UnmarshalJSON accepts a decimal JSON number using ParseRate.
// Strings, null, and exponent notation are rejected; failures leave the receiver unchanged.
func (r *Rate) UnmarshalJSON(data []byte) error {
	if r == nil || len(data) == 0 || bytes.Equal(data, []byte("null")) || data[0] == '"' {
		return ErrInvalidRate
	}

	parsed, err := ParseRate(string(data))

	if err != nil {
		return err
	}

	*r = parsed

	return nil
}

// AccumulatorPayout multiplies all rates using an unbounded intermediate and
// rounds once, at the final minor-unit payout.
// Rounding is half up. Empty rates return the original stake; an unrepresentable
// final result returns ErrOverflow.
func AccumulatorPayout(stake Money, rates []Rate) (Money, error) {
	numerator := big.NewInt(stake.minor)
	denominator := big.NewInt(1)

	for _, rate := range rates {
		numerator.Mul(numerator, big.NewInt(rate.micro))
		denominator.Mul(denominator, big.NewInt(rateScale))
	}

	return moneyFromBigRounded(numerator, denominator)
}

func moneyFromBigRounded(numerator, denominator *big.Int) (Money, error) {
	if numerator.Sign() < 0 || denominator.Sign() <= 0 {
		return Money{}, ErrInvalidMoney
	}

	q, rem := new(big.Int), new(big.Int)
	q.QuoRem(numerator, denominator, rem)

	if new(big.Int).Lsh(rem, 1).Cmp(denominator) >= 0 {
		q.Add(q, big.NewInt(1))
	}

	if !q.IsInt64() {
		return Money{}, ErrOverflow
	}

	return NewMoneyFromMinor(q.Int64())
}

func parseFixed(s string, scale int, signed bool) (int64, error) {
	// Validate the input grammar before handling its sign or decimal parts.
	if s == "" || strings.TrimSpace(s) != s || strings.ContainsAny(s, "eE+") {
		return 0, errors.New("invalid fixed-point value")
	}

	negative := strings.HasPrefix(s, "-")
	if negative {
		if !signed {
			return 0, errors.New("negative value")
		}

		s = strings.TrimPrefix(s, "-")
	}

	parts := strings.Split(s, ".")
	if len(parts) > 2 || parts[0] == "" || (len(parts) == 2 && parts[1] == "") || len(parts) == 2 && len(parts[1]) > scale {
		return 0, errors.New("invalid fixed-point value")
	}

	for _, p := range parts {
		if p == "" {
			continue
		}

		for _, c := range p {
			if c < '0' || c > '9' {
				return 0, errors.New("invalid fixed-point value")
			}
		}
	}

	frac := ""
	if len(parts) == 2 {
		frac = parts[1]
	}

	frac += strings.Repeat("0", scale-len(frac))

	combined := strings.TrimLeft(parts[0]+frac, "0")
	if combined == "" {
		return 0, nil
	}

	limit := strconv.FormatInt(math.MaxInt64, 10)

	if negative {
		limit = "9223372036854775808"
	}

	if len(combined) > len(limit) || len(combined) == len(limit) && combined > limit {
		return 0, ErrOverflow
	}

	v := new(big.Int)
	v.SetString(combined, 10)

	if negative {
		v.Neg(v)
	}

	return v.Int64(), nil
}

func formatFixed(v int64, scale int) string {
	negative := v < 0
	abs := new(big.Int).SetInt64(v)

	if negative {
		abs.Neg(abs)
	}

	digits := abs.String()

	if len(digits) <= scale {
		digits = strings.Repeat("0", scale+1-len(digits)) + digits
	}

	result := digits[:len(digits)-scale] + "." + digits[len(digits)-scale:]

	if negative {
		return "-" + result
	}

	return result
}

// GoString returns the Money representation used by the %#v format verb.
func (m Money) GoString() string { return fmt.Sprintf("Money(%s)", m.String()) }

// GoString returns the SignedMoney representation used by the %#v format verb.
func (m SignedMoney) GoString() string { return fmt.Sprintf("SignedMoney(%s)", m.String()) }
