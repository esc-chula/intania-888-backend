package model

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
	ErrInvalidMoney = errors.New("invalid money value")
	ErrInvalidRate  = errors.New("invalid rate value")
	ErrOverflow     = errors.New("fixed-point arithmetic overflow")
)

// Money is a non-negative amount stored in minor (hundredth) units.
// It deliberately has no database interfaces; persistence rows store int64s.
type Money struct {
	minor int64
}

func NewMoneyFromMinor(minor int64) (Money, error) {
	if minor < 0 {
		return Money{}, ErrInvalidMoney
	}

	return Money{minor: minor}, nil
}

func MustMoneyFromMinor(minor int64) Money {
	m, err := NewMoneyFromMinor(minor)

	if err != nil {
		panic(err)
	}

	return m
}

func ParseMoney(s string) (Money, error) {
	v, err := parseFixed(s, 2, false)

	if err != nil {
		return Money{}, ErrInvalidMoney
	}

	return NewMoneyFromMinor(v)
}

func (m Money) MinorUnits() int64 { return m.minor }
func (m Money) IsZero() bool      { return m.minor == 0 }
func (m Money) String() string    { return formatFixed(m.minor, 2) }

func (m Money) Compare(other Money) int {
	if m.minor < other.minor {
		return -1
	}

	if m.minor > other.minor {
		return 1
	}

	return 0
}

func (m Money) Greater(other Money) bool { return m.minor > other.minor }
func (m Money) Lesser(other Money) bool  { return m.minor < other.minor }

func (m Money) Add(other Money) (Money, error) {
	if other.minor > math.MaxInt64-m.minor {
		return Money{}, ErrOverflow
	}

	return Money{minor: m.minor + other.minor}, nil
}

func (m Money) Sub(other Money) (Money, error) {
	if other.minor > m.minor {
		return Money{}, ErrInvalidMoney
	}

	return Money{minor: m.minor - other.minor}, nil
}

// Mul applies a fixed-point rate and rounds half up to the nearest minor unit.
func (m Money) Mul(rate Rate) (Money, error) {
	return moneyFromBigRounded(
		new(big.Int).Mul(big.NewInt(m.minor), big.NewInt(rate.micro)),
		big.NewInt(rateScale),
	)
}

func (m Money) MarshalJSON() ([]byte, error) { return json.Marshal(m.String()) }

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
type SignedMoney struct {
	minor int64
}

func NewSignedMoneyFromMinor(minor int64) SignedMoney { return SignedMoney{minor: minor} }

func ParseSignedMoney(s string) (SignedMoney, error) {
	v, err := parseFixed(s, 2, true)

	if err != nil {
		return SignedMoney{}, ErrInvalidMoney
	}

	return SignedMoney{minor: v}, nil
}

func (m SignedMoney) MinorUnits() int64            { return m.minor }
func (m SignedMoney) String() string               { return formatFixed(m.minor, 2) }
func (m SignedMoney) MarshalJSON() ([]byte, error) { return json.Marshal(m.String()) }

func (m SignedMoney) Add(other SignedMoney) (SignedMoney, error) {
	positiveOverflow := other.minor > 0 && m.minor > math.MaxInt64-other.minor
	negativeOverflow := other.minor < 0 && m.minor < math.MinInt64-other.minor

	if positiveOverflow || negativeOverflow {
		return SignedMoney{}, ErrOverflow
	}

	return SignedMoney{minor: m.minor + other.minor}, nil
}

func (m SignedMoney) Sub(other SignedMoney) (SignedMoney, error) {
	result := new(big.Int).Sub(big.NewInt(m.minor), big.NewInt(other.minor))

	if !result.IsInt64() {
		return SignedMoney{}, ErrOverflow
	}

	return SignedMoney{minor: result.Int64()}, nil
}

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
type Rate struct {
	micro int64
}

func NewRateFromMicro(micro int64) (Rate, error) {
	if micro < 0 {
		return Rate{}, ErrInvalidRate
	}

	return Rate{micro: micro}, nil
}

func MustRateFromMicro(micro int64) Rate {
	r, err := NewRateFromMicro(micro)

	if err != nil {
		panic(err)
	}

	return r
}

func ParseRate(s string) (Rate, error) {
	v, err := parseFixed(s, 6, false)

	if err != nil {
		return Rate{}, ErrInvalidRate
	}

	return NewRateFromMicro(v)
}

func (r Rate) MicroUnits() int64 { return r.micro }
func (r Rate) String() string    { return formatFixed(r.micro, 6) }

// Rate remains a JSON number for compatibility, while calculations remain fixed-point.
func (r Rate) MarshalJSON() ([]byte, error) { return []byte(r.String()), nil }

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

func (m Money) GoString() string       { return fmt.Sprintf("Money(%s)", m.String()) }
func (m SignedMoney) GoString() string { return fmt.Sprintf("SignedMoney(%s)", m.String()) }
