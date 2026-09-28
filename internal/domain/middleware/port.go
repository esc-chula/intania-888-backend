package middleware

import (
	"context"

	"github.com/esc-chula/intania-888-backend/internal/identity"
	"github.com/esc-chula/intania-888-backend/internal/security"
)

// ServicePort describes the authentication decisions needed by HTTP middleware.
type ServicePort interface {
	// GetSession validates an opaque browser credential and renews its stored idle lifetime.
	GetSession(context.Context, string) (*security.Session, error)
	// VerifyExternalToken validates the JWT and active subject binding, returning the account identifier.
	VerifyExternalToken(context.Context, string) (string, error)
	// GetMe loads the current account profile used by downstream request handlers.
	GetMe(context.Context, string) (*identity.Profile, error)
	// IsBlacklisted checks current email and Google subject blacklist rules.
	IsBlacklisted(context.Context, string, string) (bool, error)
}

// Repository loads account snapshots required for each authenticated request.
type Repository interface {
	// GetByID loads the current account snapshot or returns identity.ErrUserNotFound.
	GetByID(context.Context, string) (*identity.User, error)
}

// SessionStore persists browser sessions and external-token subjects.
type SessionStore interface {
	// ReadAndRenewSession atomically loads a session and extends its idle TTL within the absolute lifetime.
	// The supplied current time is Unix seconds and the idle TTL is measured in seconds.
	ReadAndRenewSession(context.Context, string, int64, int) (*security.Session, error)
	// GetExternalSubject loads the active subject bound to an external JWT cache key.
	GetExternalSubject(context.Context, string) (string, error)
}
