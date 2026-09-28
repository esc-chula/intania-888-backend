package value

import (
	"encoding/json"
	"math"
	"testing"
)

func TestMoneyJSONAndParsing(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: "0", want: "0.00"},
		{input: "1.2", want: "1.20"},
		{input: "888.88", want: "888.88"},
	}

	for _, tc := range tests {
		m, err := ParseMoney(tc.input)

		if err != nil || m.String() != tc.want {
			t.Fatalf("ParseMoney(%q) = %v, %v", tc.input, m, err)
		}

		got, err := json.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}

		if string(got) != `"`+tc.want+`"` {
			t.Fatalf("json = %s", got)
		}
	}

	var m Money

	if json.Unmarshal([]byte(`1.00`), &m) == nil {
		t.Fatal("numeric money JSON must fail")
	}

	invalidValues := []string{"-1", "1.001", "1.", ".1", " 1", "1e2"}

	for _, bad := range invalidValues {
		if _, err := ParseMoney(bad); err == nil {
			t.Fatalf("ParseMoney(%q) succeeded", bad)
		}
	}
}

func TestMoneyArithmeticAndRounding(t *testing.T) {
	one := MustMoneyFromMinor(1)
	two := MustMoneyFromMinor(2)

	if !two.Greater(one) || !one.Lesser(two) || one.Greater(one) || one.Lesser(one) {
		t.Fatal("money ordering helpers returned an invalid result")
	}

	threeHalves := MustRateFromMicro(1_500_000)
	got, err := one.Mul(threeHalves)

	if err != nil || got.MinorUnits() != 2 {
		t.Fatalf("rounding tie = %v, %v", got, err)
	}

	if _, err := MustMoneyFromMinor(math.MaxInt64).Add(one); err == nil {
		t.Fatal("expected overflow")
	}

	if _, err := one.Sub(MustMoneyFromMinor(2)); err == nil {
		t.Fatal("expected negative rejection")
	}

	rates := []Rate{
		MustRateFromMicro(1_500_000),
		MustRateFromMicro(1_500_000),
	}
	acc, err := AccumulatorPayout(MustMoneyFromMinor(1_00), rates)

	if err != nil || acc.MinorUnits() != 2_25 {
		t.Fatalf("accumulator = %v, %v", acc, err)
	}
}

func TestSignedMoneyAndRateJSON(t *testing.T) {
	s, err := ParseSignedMoney("-150.00")

	if err != nil || s.String() != "-150.00" {
		t.Fatalf("signed = %v, %v", s, err)
	}

	r, err := ParseRate("1.234567")

	if err != nil {
		t.Fatal(err)
	}

	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}

	if string(b) != "1.234567" {
		t.Fatalf("rate JSON = %s", b)
	}

	if _, err := NewSignedMoneyFromMinor(math.MaxInt64).Add(NewSignedMoneyFromMinor(1)); err == nil {
		t.Fatal("expected signed overflow")
	}

	if _, err := NewSignedMoneyFromMinor(math.MinInt64).Sub(NewSignedMoneyFromMinor(1)); err == nil {
		t.Fatal("expected signed subtraction overflow")
	}
}
