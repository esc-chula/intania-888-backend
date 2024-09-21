package user

import "github.com/wiraphatys/intania888/internal/model"

type UserRepository interface {
	Create(user *model.User) error
	GetById(id string) (*model.User, error)
	GetByEmail(email string) (*model.User, error)
	GetAll() ([]model.User, error)
	Update(user *model.User) error
	Delete(id string) error
}

type UserService interface {
	CreateUser(userDto *model.UserDto) error
	GetUser(id string) (*model.User, error)
	GetAllUsers() ([]model.User, error)
	UpdateUser(userDto *model.UserDto) error
	DeleteUser(id string) error
}
