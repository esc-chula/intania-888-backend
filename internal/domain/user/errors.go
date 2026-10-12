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
	// ErrInvalidProfileUpdate indicates an empty update or an invalid editable field.
	ErrInvalidProfileUpdate = errors.New("invalid profile update")
	// ErrInvalidProfileRegistration indicates missing registration fields.
	ErrInvalidProfileRegistration = errors.New("invalid profile registration")
	// ErrProfileGroupNotFound indicates that a profile update refers to an unknown group.
	ErrProfileGroupNotFound = errors.New("profile group not found")
	// ErrGroupAlreadyAssigned indicates registration attempted to replace an existing group.
	ErrGroupAlreadyAssigned = errors.New("group already assigned")
	// ErrGroupRegistrationConflict indicates concurrent profile changes prevented registration.
	ErrGroupRegistrationConflict = errors.New("group registration conflicts with current profile")
)
