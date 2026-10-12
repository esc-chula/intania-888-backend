package identity

import (
	"errors"
	"time"

	"github.com/esc-chula/intania-888-backend/internal/value"
)

// ErrUserNotFound identifies an account that no longer exists.
var ErrUserNotFound = errors.New("user not found")

// User is an account snapshot used by authentication and account use cases.
// Balances are stored in minor units at this internal boundary.
// Nil NickName and GroupID preserve the absence of those optional account values.
type User struct {
	ID            string
	Email         string
	Name          string
	NickName      *string
	RoleID        string
	GroupID       *string
	RemainingCoin int64
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// Profile is the authenticated actor and current account state.
// It contains no transport or persistence metadata.
// RoleID and RemainingCoin are snapshots of the account at the time it was loaded.
type Profile struct {
	ID            string
	Email         string
	Name          string
	NickName      *string
	RoleID        string
	GroupID       *string
	RemainingCoin value.Money
	CreatedAt     time.Time
}
