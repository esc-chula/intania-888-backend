package value

import (
	"encoding/json"
	"errors"
	"math"
	"testing"
)

func TestRateJSONCanonicalOutputAndRoundTrip(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		canonical string
		micro     int64
	}{
		{"zero", `"0"`, `"0.000000"`, 0},
		{"integer", `"12"`, `"12.000000"`, 12_000_000},
		{"short fraction", `"1.75"`, `"1.750000"`, 1_750_000},
		{"leading zeros", `"001.20"`, `"1.200000"`, 1_200_000},
		{"smallest unit", `"0.000001"`, `"0.000001"`, 1},
		{"six digits", `"1.234567"`, `"1.234567"`, 1_234_567},
		{"escaped digits", `"\u0031.\u0032"`, `"1.200000"`, 1_200_000},
		{"beyond JavaScript safe integer", `"9007199254.740993"`, `"9007199254.740993"`, 9_007_199_254_740_993},
		{"maximum", `"9223372036854.775807"`, `"9223372036854.775807"`, math.MaxInt64},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var rate Rate
			if err := json.Unmarshal([]byte(test.input), &rate); err != nil {
				t.Fatal(err)
			}
			if rate.MicroUnits() != test.micro {
				t.Fatalf("micro units = %d; want %d", rate.MicroUnits(), test.micro)
			}

			encoded, err := json.Marshal(rate)
			if err != nil {
				t.Fatal(err)
			}
			if string(encoded) != test.canonical {
				t.Fatalf("JSON = %s; want %s", encoded, test.canonical)
			}

			var decoded Rate
			if err := json.Unmarshal(encoded, &decoded); err != nil {
				t.Fatal(err)
			}
			if decoded.MicroUnits() != rate.MicroUnits() {
				t.Fatalf("round trip lost precision: %d became %d", rate.MicroUnits(), decoded.MicroUnits())
			}
		})
	}

	encoded, err := json.Marshal(Rate{})
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded) != `"0.000000"` {
		t.Fatalf("zero value JSON = %s", encoded)
	}
}

func TestRateJSONRejectsInvalidValuesWithoutChangingReceiver(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"numeric zero", `0`},
		{"numeric decimal", `1.75`},
		{"numeric exponent", `1e2`},
		{"null", `null`},
		{"boolean", `true`},
		{"array", `[]`},
		{"object", `{}`},
		{"empty string", `""`},
		{"negative", `"-1.000000"`},
		{"negative zero", `"-0"`},
		{"plus sign", `"+1"`},
		{"quoted exponent", `"1e2"`},
		{"leading whitespace", `" 1"`},
		{"trailing whitespace", `"1 "`},
		{"internal whitespace", `"1 .2"`},
		{"escaped whitespace", `"1\n"`},
		{"excess precision", `"1.0000001"`},
		{"missing integer", `".1"`},
		{"missing fraction", `"1."`},
		{"comma", `"1,000"`},
		{"overflow", `"9223372036854.775808"`},
		{"integer overflow", `"9223372036855"`},
		{"malformed string", `"1`},
		{"empty input", ``},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rate := MustRateFromMicro(1_750_000)
			if err := rate.UnmarshalJSON([]byte(test.input)); !errors.Is(err, ErrInvalidRate) {
				t.Fatalf("error = %v; want ErrInvalidRate", err)
			}
			if rate.MicroUnits() != 1_750_000 {
				t.Fatalf("invalid JSON changed receiver to %d", rate.MicroUnits())
			}
		})
	}

	var rate *Rate
	if err := rate.UnmarshalJSON([]byte(`"1"`)); !errors.Is(err, ErrInvalidRate) {
		t.Fatalf("nil receiver error = %v; want ErrInvalidRate", err)
	}
}
