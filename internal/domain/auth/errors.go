package auth

import "errors"

// Authentication failures identify invalid protocol state or disallowed identity.
var (
	// ErrLegacyExternalTokensRetired indicates that the migration window is closed.
	ErrLegacyExternalTokensRetired = errors.New("legacy external tokens are retired")

	// ErrInvalidOAuthState indicates missing, mismatched, expired, or previously consumed login state.
	ErrInvalidOAuthState = errors.New("invalid OAuth state")
	// ErrUnverifiedEmail indicates that Google has not verified the account email.
	ErrUnverifiedEmail = errors.New("google email is not verified")
	// ErrEmailNotAllowed indicates that the account fails the shared login access policy.
	ErrEmailNotAllowed = errors.New("email is not allowed")
	// ErrInvalidSession indicates an invalid browser session credential.
	ErrInvalidSession = errors.New("invalid session")
)
