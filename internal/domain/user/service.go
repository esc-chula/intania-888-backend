package user

import (
	"errors"

	"github.com/esc-chula/intania-888-backend/internal/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type userServiceImpl struct {
	repo UserRepository
	db   *gorm.DB
	log  *zap.Logger
}

func NewUserService(repo UserRepository, db *gorm.DB, log *zap.Logger) UserService {
	return &userServiceImpl{
		repo: repo,
		db:   db,
		log:  log,
	}
}

func (s *userServiceImpl) CreateUser(userDto *model.UserDto) error {
	userDto.RemainingCoin = 888.00
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
		Id:            user.Id,
		Email:         user.Email,
		Name:          user.Name,
		RoleId:        user.RoleId,
		RemainingCoin: user.RemainingCoin,
		NickName:      user.NickName,
		GroupId:       user.GroupId,
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
			Id:            user.Id,
			Email:         user.Email,
			Name:          user.Name,
			RoleId:        user.RoleId,
			RemainingCoin: user.RemainingCoin,
			NickName:      user.NickName,
			GroupId:       user.GroupId,
		}
	}

	s.log.Named("GetAllUsers").Info("Successfully fetched all users", zap.Int("count", len(users)))
	return usersDto, nil
}

func (s *userServiceImpl) UpdateUser(userDto *model.UserDto) error {
	existed, err := s.repo.GetById(userDto.Id)
	if err != nil {
		s.log.Named("UpdateUser").Error("Failed to get existed user", zap.Error(err))
		return err
	}
	userDto.RemainingCoin = existed.RemainingCoin

	err = s.repo.Update(ToUserEntity(userDto))
	if err != nil {
		s.log.Named("UpdateUser").Error("Failed to update user", zap.Error(err))
		return err
	}

	s.log.Named("UpdateUser").Info("User updated successfully", zap.String("user_id", userDto.Id))
	return nil
}

func (s *userServiceImpl) AdminUpdateUser(userId string, userDto *model.UserDto) error {
	existed, err := s.repo.GetById(userId)
	if err != nil {
		s.log.Named("AdminUpdateUser").Error("Failed to get existed user", zap.Error(err))
		return err
	}

	// Admin can update all fields including role and coins
	existed.Name = userDto.Name
	existed.NickName = userDto.NickName
	existed.RoleId = userDto.RoleId
	existed.RemainingCoin = userDto.RemainingCoin
	if userDto.GroupId != nil {
		existed.GroupId = userDto.GroupId
	}

	err = s.repo.Update(existed)
	if err != nil {
		s.log.Named("AdminUpdateUser").Error("Failed to update user", zap.Error(err))
		return err
	}

	s.log.Named("AdminUpdateUser").Info("User updated by admin successfully", zap.String("user_id", userId))
	return nil
}

// DeductCoin deducts coins from user balance atomically with transaction safety
func (s *userServiceImpl) DeductCoin(userId string, amount float64) (float64, error) {
	var remainingBalance float64

	err := s.db.Transaction(func(tx *gorm.DB) error {
		// 1. Lock user row for update
		var user model.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", userId).
			First(&user).Error; err != nil {
			s.log.Named("DeductCoin").Error("User not found", zap.Error(err))
			return errors.New("user not found")
		}

		// 2. Validate balance (allow exactly 0, reject negative)
		if user.RemainingCoin < amount {
			s.log.Named("DeductCoin").Warn("Insufficient balance",
				zap.String("userId", userId),
				zap.Float64("balance", user.RemainingCoin),
				zap.Float64("amount", amount))
			return errors.New("insufficient balance")
		}

		// 3. Atomic deduction using SQL expression
		if err := tx.Model(&model.User{}).
			Where("id = ?", userId).
			Update("remaining_coin", gorm.Expr("remaining_coin - ?", amount)).
			Error; err != nil {
			s.log.Named("DeductCoin").Error("Failed to deduct coins", zap.Error(err))
			return errors.New("failed to deduct coins")
		}

		// 4. Calculate remaining balance for response
		remainingBalance = user.RemainingCoin - amount

		s.log.Named("DeductCoin").Info("Coins deducted successfully",
			zap.String("userId", userId),
			zap.Float64("amount", amount),
			zap.Float64("remaining", remainingBalance))

		return nil
	})

	if err != nil {
		return 0, err
	}

	return remainingBalance, nil
}
