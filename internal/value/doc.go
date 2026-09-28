// Package value provides exact fixed-point money and rate arithmetic.
// Money and SignedMoney use hundredth units, while Rate uses millionth units.
// Amounts marshal as decimal JSON strings and rates as decimal JSON numbers;
// calculations use integer arithmetic with checked results and half-up rounding.
package value
