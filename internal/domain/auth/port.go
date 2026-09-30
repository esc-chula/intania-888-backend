package auth

import (
	"context"

	"github.com/esc-chula/intania-888-backend/internal/identity"
	"github.com/esc-chula/intania-888-backend/internal/security"
)

// ServicePort describes the authentication use cases required by HTTP handlers.
type ServicePort interface {
	// StartOAuthLogin creates one-use OAuth state and returns its browser authorization URL.
	StartOAuthLogin(context.Context) (*OAuthLogin, error)
	// VerifyOAuthLogin consumes bound OAuth state, checks access policy, and rotates a browser session.
	VerifyOAuthLogin(context.Context, string, string, string, string) (*SessionCredentials, error)
	// Logout revokes the session identified by its opaque ID; absent sessions already count as logged out.
	Logout(context.Context, string) error
	// GetPostLoginRedirectURL returns the configured fixed frontend destination.
	GetPostLoginRedirectURL() string
	// IssueExternalToken returns a JWT, revocation ID, and configured lifetime for an existing subject.
	IssueExternalToken(context.Context, string) (*IssuedExternalToken, error)
	// RevokeExternalToken removes the active subject binding for a token identifier.
	RevokeExternalToken(context.Context, string) error
}

// Repository stores authentication protocol state without exposing cache encoding.
type Repository interface {
	// StoreOAuthState persists the PKCE verifier under an opaque-state cache key; ttl is in seconds.
	StoreOAuthState(context.Context, string, OAuthState, int) error
	// ConsumeOAuthState atomically reads and deletes state, returning ErrInvalidOAuthState when absent.
	ConsumeOAuthState(context.Context, string) (OAuthState, error)
	// RotateSession atomically stores the new session and revokes its predecessor.
	// The idle and absolute lifetimes are measured in seconds.
	RotateSession(context.Context, string, string, string, security.Session, int, int) error
	// DeleteSession idempotently removes the record identified by its cache key.
	DeleteSession(context.Context, string) error
	// StoreExternalToken records the subject authorized by a JWT identifier; ttl is in seconds.
	StoreExternalToken(context.Context, string, string, int) error
}

// Users is the account persistence boundary required during authentication.
type Users interface {
	// GetByID loads an account snapshot or returns identity.ErrUserNotFound.
	GetByID(context.Context, string) (*identity.User, error)
	// GetByEmail loads an account by its normalized email or returns identity.ErrUserNotFound.
	GetByEmail(context.Context, string) (*identity.User, error)
	// Create persists the newly admitted account.
	Create(context.Context, *identity.User) error
}

// SessionReader reads the browser session needed to authorize logout.
type SessionReader interface {
	// Session loads the browser session needed for logout and preserves missing-session errors.
	Session(context.Context, string) (*security.Session, error)
}
