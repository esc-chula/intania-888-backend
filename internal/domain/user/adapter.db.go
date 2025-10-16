package user

import (
	"github.com/esc-chula/intania-888-backend/internal/model"
	"gorm.io/gorm"
)

type userRepositoryImpl struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepositoryImpl{db: db}
}

func (r *userRepositoryImpl) Create(user *model.User) error {
	return r.db.Create(user).Error
}

func (r *userRepositoryImpl) GetById(id string) (*model.User, error) {
	var user model.User
	if err := r.db.Preload("Role").Where("id = ?", id).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepositoryImpl) GetByEmail(email string) (*model.User, error) {
	var user model.User
	if err := r.db.Preload("Role").Where("email = ?", email).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepositoryImpl) GetAll() ([]*model.User, error) {
	var users []*model.User
	if err := r.db.Preload("Role").Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

func (r *userRepositoryImpl) Update(user *model.User) error {
	return r.db.Model(user).Where("id = ?", user.Id).Updates(user).Error
}
