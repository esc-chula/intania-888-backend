package bill

import (
	"github.com/esc-chula/intania-888-backend/internal/model"
	"gorm.io/gorm"
)

type billRepositoryImpl struct {
	db *gorm.DB
}

func NewBillRepository(db *gorm.DB) BillRepository {
	return &billRepositoryImpl{db: db}
}

func preload(db *gorm.DB) *gorm.DB {
	return db.
		Preload("Lines").
		Preload("Lines.Match")
}

func (r *billRepositoryImpl) GetById(id, userID string) (*model.BillHead, error) {
	var v model.BillHead
	q := preload(r.db).Where("id = ?", id)

	if userID != "" {
		q = q.Where("user_id = ?", userID)
	}

	if err := q.First(&v).Error; err != nil {
		return nil, err
	}

	return &v, nil
}

func (r *billRepositoryImpl) GetAll(userID string) ([]*model.BillHead, error) {
	var v []*model.BillHead

	err := preload(r.db).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&v).
		Error

	return v, err
}

func (r *billRepositoryImpl) GetAllAdmin() ([]*model.BillHead, error) {
	var v []*model.BillHead

	err := preload(r.db).
		Preload("User").
		Order("created_at DESC").
		Find(&v).
		Error

	return v, err
}
