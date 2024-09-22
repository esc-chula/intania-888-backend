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
	err := s.repo.Create(ToUserEntity(userDto))
	if err != nil {
		s.log.Named("CreateUser").Error("Failed to create user", zap.Error(err))
		return err
	}

	s.log.Named("CreateUser").Info("User created successfully", zap.String("email", userDto.Email))
	return nil
}

func (s *userServiceImpl) GetUser(id string) (*model.UserDto, error) {
	user, err := s.repo.GetById(id)
	if err != nil {
		s.log.Named("GetUser").Error("Get by id", zap.Error(err))
		return nil, err
	}

	s.log.Named("GetUser").Info("Successfully fetched user by id", zap.String("user_id", user.Id))
	return &model.UserDto{
		Id:     user.Id,
		Email:  user.Email,
		Name:   user.Name,
		RoleId: user.RoleId,
	}, nil
}

func (s *userServiceImpl) GetAllUsers() ([]*model.UserDto, error) {
	users, err := s.repo.GetAll()
	if err != nil {
		s.log.Named("GetAllUsers").Error("Failed to fetch users", zap.Error(err))
		return nil, err
	}

	usersDto := make([]*model.UserDto, len(users))
	for i, user := range users {
		usersDto[i] = &model.UserDto{
			Id:     user.Id,
			Email:  user.Email,
			Name:   user.Name,
			RoleId: user.RoleId,
		}
	}

	s.log.Named("GetAllUsers").Info("Successfully fetched all users", zap.Int("count", len(users)))
	return usersDto, nil
}

func (s *userServiceImpl) UpdateUser(userDto *model.UserDto) error {
	err := s.repo.Update(ToUserEntity(userDto))
	if err != nil {
		s.log.Named("UpdateUser").Error("Failed to update user", zap.Error(err))
		return err
	}

	s.log.Named("UpdateUser").Info("User updated successfully", zap.String("user_id", userDto.Id))
	return nil
}

func (s *userServiceImpl) DeleteUser(id string) error {
	err := s.repo.Delete(id)
	if err != nil {
		s.log.Named("DeleteUser").Error("Failed to delete user", zap.Error(err))
		return err
	}

	s.log.Named("DeleteUser").Info("User deleted successfully", zap.String("user_id", id))
	return nil
}
