package auth

import "github.com/esc-chula/intania-888-backend/internal/model"

type AuthService interface {
	GetOAuthUrl() (string, error)
	VerifyOAuthLogin(code string) (*model.CredentialDto, error)
	RefreshToken(refreshToken string) (*model.CredentialDto, error)
}

type AuthRepository interface {
	SetCacheValue(key string, value interface{}, ttl int) error
	GetCacheValue(key string, value interface{}) error
}
