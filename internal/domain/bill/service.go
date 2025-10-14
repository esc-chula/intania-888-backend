package bill

import (
	"errors"

	"github.com/esc-chula/intania-888-backend/internal/domain/user"
	"github.com/esc-chula/intania-888-backend/internal/model"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type billServiceImpl struct {
	repo     BillRepository
	userRepo user.UserRepository
	log      *zap.Logger
	db       *gorm.DB
}

// Create a new instance of BillService
func NewBillService(repo BillRepository, userRepo user.UserRepository, log *zap.Logger, db *gorm.DB) BillService {
	return &billServiceImpl{repo, userRepo, log, db}
}

// CreateBill creates a new bill with transaction
func (s *billServiceImpl) CreateBill(userProfile *model.UserDto, billDto *model.BillHeadDto) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var user model.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", userProfile.Id).
			First(&user).Error; err != nil {
			s.log.Named("CreateBill").Error("Get user by Id with lock", zap.Error(err))
			return err
		}

		if user.RemainingCoin < billDto.Total {
			err := errors.New("user does not have enough coins to cover the total bill")
			s.log.Named("CreateBill").Error("Check for balance", zap.Error(err))
			return err
		}

		bill := mapBillDtoToEntity(billDto)
		bill.Id = uuid.NewString()
		for i := range bill.Lines {
			bill.Lines[i].BillId = bill.Id
		}

		if err := tx.Create(bill).Error; err != nil {
			s.log.Named("CreateBill").Error("Create bill", zap.Error(err))
			return err
		}

		if err := tx.Model(&model.User{}).
			Where("id = ?", user.Id).
			Update("remaining_coin", gorm.Expr("remaining_coin - ?", bill.Total)).
			Error; err != nil {
			s.log.Named("CreateBill").Error("Update user balance", zap.Error(err))
			return err
		}

		s.log.Named("CreateBill").Info("Created bill successful", zap.Any("bill", bill))
		return nil
	})
}

// GetBill returns a bill by id
func (s *billServiceImpl) GetBill(billId, userId string) (*model.BillHeadDto, error) {
	bill, err := s.repo.GetById(billId, userId)
	if err != nil {
		s.log.Named("GetBill").Error("GetById", zap.Error(err))
		return nil, err
	}

	if bill == nil {
		s.log.Named("GetBill").Error("Bill not found", zap.String("id", billId))
		return nil, errors.New("bill not found")
	}

	billDto := mapBillEntityToDto(bill)
	s.log.Named("GetBill").Info("Retrieved bill successful", zap.String("id", billId))
	return billDto, nil
}

// GetAllBills returns all bills for a specific user
func (s *billServiceImpl) GetAllBills(userId string) ([]*model.BillHeadDto, error) {
	bills, err := s.repo.GetAll(userId)
	if err != nil {
		s.log.Named("GetAllBills").Error("GetAll", zap.Error(err))
		return nil, err
	}

	billDtos := mapBillsEntityToDto(bills)
	s.log.Named("GetAllBills").Info("Retrieved all bills successful", zap.Int("count", len(billDtos)))
	return billDtos, nil
}

// GetAllBillsAdmin returns all bills from all users (admin only)
func (s *billServiceImpl) GetAllBillsAdmin() ([]*model.BillHeadDto, error) {
	bills, err := s.repo.GetAllAdmin()
	if err != nil {
		s.log.Named("GetAllBillsAdmin").Error("GetAllAdmin", zap.Error(err))
		return nil, err
	}

	billDtos := mapBillsEntityToDto(bills)
	s.log.Named("GetAllBillsAdmin").Info("Retrieved all bills (admin) successful", zap.Int("count", len(billDtos)))
	return billDtos, nil
}

// UpdateBill updates an existing bill
func (s *billServiceImpl) UpdateBill(billDto *model.BillHeadDto) error {
	bill := mapBillDtoToEntity(billDto)

	err := s.repo.Update(bill)
	if err != nil {
		s.log.Named("UpdateBill").Error("Update", zap.Error(err))
		return err
	}

	s.log.Named("UpdateBill").Info("Updated bill successful", zap.String("id", bill.Id))
	return nil
}

// DeleteBill deletes a bill by id
func (s *billServiceImpl) DeleteBill(id string) error {
	err := s.repo.Delete(id)
	if err != nil {
		s.log.Named("DeleteBill").Error("Delete", zap.Error(err))
		return err
	}

	s.log.Named("DeleteBill").Info("Deleted bill successful", zap.String("id", id))
	return nil
}
