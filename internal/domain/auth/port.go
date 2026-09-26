package auth

import "errors"

var (
	ErrInvalidOAuthState = errors.New("invalid OAuth state")
	ErrUnverifiedEmail   = errors.New("google email is not verified")
	ErrEmailNotAllowed   = errors.New("email is not allowed")
	ErrInvalidRefresh    = errors.New("invalid refresh token")
	ErrRefreshReplay     = errors.New("refresh token replay detected")
)

type OAuthLogin struct {
	URL   string
	State string
}

type SessionCredentials struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int32
	IsNewUser    bool
}

type AuthService interface {
	StartOAuthLogin() (*OAuthLogin, error)
	VerifyOAuthLogin(code, state, cookieState string) (*SessionCredentials, error)
	RefreshToken(refreshToken string) (*SessionCredentials, error)
	Logout(sessionID string) error
	GetPostLoginRedirectURL() string
}

type AuthRepository interface {
	SetCacheValue(key string, value interface{}, ttl int) error
	GetCacheValue(key string, value interface{}) error
	DeleteCacheValue(key string) error
	DeleteCacheValues(keys ...string) error
	ConsumeCacheValue(key string, value interface{}) error
	CompareAndSwapCacheValues(expected map[string]interface{}, replacements map[string]interface{}, ttl int) (bool, error)
}
