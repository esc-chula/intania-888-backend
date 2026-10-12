package policy

import "errors"

var (
	// ErrInvalidPolicy indicates invalid policy input or filters.
	ErrInvalidPolicy = errors.New("invalid access policy")
	// ErrPolicyNotFound indicates that an access policy does not exist.
	ErrPolicyNotFound = errors.New("access policy not found")
	// ErrPolicyConflict indicates a duplicate policy identity.
	ErrPolicyConflict = errors.New("access policy already exists")
)
