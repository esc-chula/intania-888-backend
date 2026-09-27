package auth

import (
	"errors"
	"github.com/esc-chula/intania-888-backend/internal/model"
)

var (
	ErrInvalidOAuthState = errors.New("invalid OAuth state")
	ErrUnverifiedEmail   = errors.New("google email is not verified")
	ErrEmailNotAllowed   = errors.New("email is not allowed")
	ErrInvalidSession    = errors.New("invalid session")
)

type OAuthLogin struct{ URL, State string }
type SessionCredentials struct {
	SessionID string
	IsNewUser bool
}

type AuthService interface {
	StartOAuthLogin() (*OAuthLogin, error)
	VerifyOAuthLogin(code, state, cookieState, previousSessionID string) (*SessionCredentials, error)
	Logout(sessionID string) error
	GetPostLoginRedirectURL() string
	IssueExternalToken(subjectID string) (string, string, error)
	RevokeExternalToken(jti string) error
}

type AuthRepository interface {
	SetCacheValue(key string, value interface{}, ttl int) error
	ConsumeCacheValue(key string, value interface{}) error
	RotateSession(
		userKey, sessionKey, previousKey string,
		value interface{},
		idleTTLSeconds, absoluteTTLSeconds int,
	) error
	DeleteSession(key string) error
	GetCacheValue(key string, value interface{}) error
}

type BrowserSessionStore interface {
	ReadAndRenewSession(key string, now int64, idleSeconds int, value interface{}) error
}

type ExternalTokenRecord struct {
	SubjectID string `json:"subject_id"`
}

var _ = model.SessionRecord{}
