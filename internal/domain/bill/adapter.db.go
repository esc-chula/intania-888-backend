package bill

import (
	"github.com/wiraphatys/intania888/internal/model"
	"gorm.io/gorm"
)

type billRepositoryImpl struct {
	db *gorm.DB
}

// NewBillRepository creates a new BillRepository instance
func NewBillRepository(db *gorm.DB) BillRepository {
	return &billRepositoryImpl{db}
}

// Create a new bill
func (r *billRepositoryImpl) Create(bill *model.BillHead) error {
	return r.db.Create(bill).Error
}

// GetById retrieves a bill by its ID
func (r *billRepositoryImpl) GetById(id string) (*model.BillHead, error) {
	var bill model.BillHead
	err := r.db.First(&bill, "id = ?", id).Preload("BillLine").Error
	if err != nil {
		return nil, err
	}
	return &bill, nil
}

// GetAll retrieves all bills
func (r *billRepositoryImpl) GetAll(userId string) ([]*model.BillHead, error) {
	var bills []*model.BillHead
	err := r.db.Preload("Lines").Preload("Lines.Color").Preload("Lines.Match.SportType").Where("user_id = ?", userId).Find(&bills).Error
	if err != nil {
		return nil, err
	}
	return bills, nil
}

// Update an existing bill
func (r *billRepositoryImpl) Update(bill *model.BillHead) error {
	return r.db.Model(bill).Where("id = ?", bill.Id).Updates(bill).Error
}

// Delete a bill by its ID
func (r *billRepositoryImpl) Delete(id string) error {
	return r.db.Delete(&model.BillHead{}, "id = ?", id).Error
}
