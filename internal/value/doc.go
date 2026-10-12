// Package value provides exact fixed-point money and rate arithmetic.
// Money and SignedMoney use hundredth units, while Rate uses millionth units.
// Amounts marshal as decimal JSON strings with two fractional digits, and rates
// as decimal JSON strings with six fractional digits. Calculations use integer
// arithmetic with checked results and half-up rounding.
package value
