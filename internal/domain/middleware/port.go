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
	// VerifyScopedExternalToken validates the active grant and required endpoint scope.
	VerifyScopedExternalToken(context.Context, string, string) (string, error)
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

// SessionStore persists browser sessions.
type SessionStore interface {
	// ReadAndRenewSession atomically loads a session and extends its idle TTL within the absolute lifetime.
	// The supplied current time is Unix seconds and the idle TTL is measured in seconds.
	ReadAndRenewSession(context.Context, string, int64, int) (*security.Session, error)
}

// DelegationReader loads active application grants with bounded idle renewal.
type DelegationReader interface {
	// ReadDelegation loads a grant and maps expiry to an authentication error.
	ReadDelegation(context.Context, string, int) (*security.Delegation, error)
}
