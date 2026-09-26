package middleware

import (
	"github.com/esc-chula/intania-888-backend/internal/model"
	"github.com/esc-chula/intania-888-backend/utils"
)

type MiddlewareService interface {
	VerifyToken(token string) (*utils.AccessTokenClaims, error)
	GetMe(userId string) (*model.UserDto, error)
	IsBlacklisted(email, userID string) (bool, error)
}

type MiddlewareRepository interface {
	GetById(id string) (*model.User, error)
}
