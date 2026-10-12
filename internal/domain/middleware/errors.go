package middleware

import "errors"

// Session and external credential failures represent missing, expired or invalid credentials.
var (
	// ErrSessionMissing indicates a missing, malformed, expired, or incomplete browser session.
	ErrSessionMissing = errors.New("session missing")
	// ErrExternalMissing indicates an invalid JWT or an absent external subject binding.
	ErrExternalMissing = errors.New("external token missing")
)
