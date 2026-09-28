package user

import (
	"errors"

	"github.com/esc-chula/intania-888-backend/internal/identity"
)

var (
	// ErrUserNotFound indicates that the requested account does not exist.
	ErrUserNotFound = identity.ErrUserNotFound
	// ErrInsufficientBalance indicates that the account cannot fund a deduction.
	ErrInsufficientBalance = errors.New("insufficient balance")
)
