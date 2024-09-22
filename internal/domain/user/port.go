package user

import "github.com/esc-chula/intania-888-backend/internal/model"

type UserRepository interface {
	Create(user *model.User) error
	GetById(id string) (*model.User, error)
	GetByEmail(email string) (*model.User, error)
	GetAll() ([]*model.User, error)
	Update(user *model.User) error
	Delete(id string) error
}

type UserService interface {
	CreateUser(userDto *model.UserDto) error
	GetUser(id string) (*model.UserDto, error)
	GetAllUsers() ([]*model.UserDto, error)
	UpdateUser(userDto *model.UserDto) error
	DeleteUser(id string) error
}
