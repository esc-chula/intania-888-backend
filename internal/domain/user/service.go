package user

import (
	"github.com/wiraphatys/intania888/internal/model"
	"go.uber.org/zap"
)

type userServiceImpl struct {
	repo UserRepository
	log  *zap.Logger
}

func NewUserService(repo UserRepository, log *zap.Logger) UserService {
	return &userServiceImpl{
		repo: repo,
		log:  log,
	}
}

func (s *userServiceImpl) CreateUser(userDto *model.UserDto) error {
	return s.repo.Create(ToUserEntity(userDto))
}

func (s *userServiceImpl) GetUser(id string) (*model.User, error) {
	return s.repo.GetById(id)
}

func (s *userServiceImpl) GetAllUsers() ([]model.User, error) {
	return s.repo.GetAll()
}

func (s *userServiceImpl) UpdateUser(userDto *model.UserDto) error {
	return s.repo.Update(ToUserEntity(userDto))
}

func (s *userServiceImpl) DeleteUser(id string) error {
	return s.repo.Delete(id)
}
