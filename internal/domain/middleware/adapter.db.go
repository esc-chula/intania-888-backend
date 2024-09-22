package middleware

import (
	"github.com/esc-chula/intania-888-backend/internal/model"
	"gorm.io/gorm"
)

type middlewareRepositoryImpl struct {
	db *gorm.DB
}

func NewMiddlewareRepository(db *gorm.DB) MiddlewareRepository {
	return &middlewareRepositoryImpl{db}
}

func (r *middlewareRepositoryImpl) GetById(id string) (*model.User, error) {
	var user model.User
	if err := r.db.Preload("Role").Where("id = ?", id).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}
