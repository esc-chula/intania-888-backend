// Package identity contains account and authenticated-actor snapshots shared
// by feature use cases. These types carry neither HTTP nor ORM metadata.
// User balances cross the account repository boundary as int64 hundredth units;
// Profile exposes the same balance as a checked value.Money amount.
package identity
