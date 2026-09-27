package middleware

import (
	"github.com/esc-chula/intania-888-backend/internal/model"
)

type MiddlewareService interface {
	GetSession(sessionID string) (*model.SessionRecord, error)
	VerifyExternalToken(token string) (string, error)
	GetMe(userID string) (*model.UserDto, error)
	IsBlacklisted(email, userID string) (bool, error)
}

type MiddlewareRepository interface {
	GetById(id string) (*model.User, error)
}
