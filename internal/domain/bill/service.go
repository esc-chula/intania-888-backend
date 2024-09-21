package bill

import (
	"errors"

	"github.com/wiraphatys/intania888/internal/model"
	"go.uber.org/zap"
)

type billServiceImpl struct {
	repo BillRepository
	log  *zap.Logger
}

// Create a new instance of BillService
func NewBillService(repo BillRepository, log *zap.Logger) BillService {
	return &billServiceImpl{repo, log}
}

// CreateBill creates a new bill
func (s *billServiceImpl) CreateBill(billDto *model.BillHeadDto) error {
	bill := mapBillDtoToEntity(billDto)

	err := s.repo.Create(bill)
	if err != nil {
		s.log.Named("CreateBill").Error("Create", zap.Error(err))
		return err
	}

	s.log.Named("CreateBill").Info("Created bill successful", zap.Any("bill", bill))
	return nil
}

// GetBill returns a bill by id
func (s *billServiceImpl) GetBill(id string) (*model.BillHeadDto, error) {
	bill, err := s.repo.GetById(id)
	if err != nil {
		s.log.Named("GetBill").Error("GetById", zap.Error(err))
		return nil, err
	}

	if bill == nil {
		s.log.Named("GetBill").Error("Bill not found", zap.String("id", id))
		return nil, errors.New("bill not found")
	}

	billDto := mapBillEntityToDto(bill)
	s.log.Named("GetBill").Info("Retrieved bill successful", zap.String("id", id))
	return billDto, nil
}

// GetAllBills returns all bills
func (s *billServiceImpl) GetAllBills() ([]*model.BillHeadDto, error) {
	bills, err := s.repo.GetAll()
	if err != nil {
		s.log.Named("GetAllBills").Error("GetAll", zap.Error(err))
		return nil, err
	}

	billDtos := mapBillsEntityToDto(bills)
	s.log.Named("GetAllBills").Info("Retrieved all bills successful", zap.Int("count", len(billDtos)))
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