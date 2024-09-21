package middleware

import "github.com/wiraphatys/intania888/internal/model"

type MiddlewareService interface {
	VerifyToken(token string) (*string, error)
	GetMe(userId string) (*model.UserDto, error)
}

type MiddlewareRepository interface {
	GetById(id string) (*model.User, error)
}
