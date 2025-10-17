// package bill

// import (
// 	"github.com/esc-chula/intania-888-backend/internal/model"
// 	"gorm.io/gorm"
// )

// type billRepositoryImpl struct {
// 	db *gorm.DB
// }

// // NewBillRepository creates a new BillRepository instance
// func NewBillRepository(db *gorm.DB) BillRepository {
// 	return &billRepositoryImpl{db}
// }

// // Create a new bill
// func (r *billRepositoryImpl) Create(bill *model.BillHead) error {
// 	return r.db.Create(bill).Error
// }

// // GetById retrieves a bill by its ID
// func (r *billRepositoryImpl) GetById(billId, userId string) (*model.BillHead, error) {
// 	var bill model.BillHead
// 	err := r.db.Preload("Lines").Preload("Lines.Match").Where("id = ? AND user_id = ?", billId, userId).First(&bill).Error
// 	if err != nil {
// 		return nil, err
// 	}
// 	return &bill, nil
// }

// // GetAll retrieves all bills for a specific user
// func (r *billRepositoryImpl) GetAll(userId string) ([]*model.BillHead, error) {
// 	var bills []*model.BillHead
// 	err := r.db.Preload("Lines").Preload("Lines.Match").Where("user_id = ?", userId).Find(&bills).Error
// 	if err != nil {
// 		return nil, err
// 	}
// 	return bills, nil
// }

// // GetAllAdmin retrieves all bills from all users (admin only)
// func (r *billRepositoryImpl) GetAllAdmin() ([]*model.BillHead, error) {
// 	var bills []*model.BillHead
// 	err := r.db.Preload("Lines").Preload("Lines.Match").Preload("User").Find(&bills).Error
// 	if err != nil {
// 		return nil, err
// 	}
// 	return bills, nil
// }

// // Update an existing bill
// func (r *billRepositoryImpl) Update(bill *model.BillHead) error {
// 	return r.db.Model(bill).Where("id = ?", bill.Id).Updates(bill).Error
// }

// // Delete a bill by its ID
// func (r *billRepositoryImpl) Delete(id string) error {
// 	return r.db.Delete(&model.BillHead{}, "id = ?", id).Error
// }

package bill

import (
	"github.com/esc-chula/intania-888-backend/internal/model"
	"gorm.io/gorm"
)

type billRepositoryImpl struct {
	db *gorm.DB
}

// NewBillRepository creates a new BillRepository instance
func NewBillRepository(db *gorm.DB) BillRepository {
	return &billRepositoryImpl{db}
}

// Create a new bill (keep GORM for struct mapping)
func (r *billRepositoryImpl) Create(bill *model.BillHead) error {
	return r.db.Create(bill).Error
}

// GetById retrieves a bill by its ID and user ID
func (r *billRepositoryImpl) GetById(billId, userId string) (*model.BillHead, error) {
	var bill model.BillHead

	// Use db.Raw for main query
	err := r.db.Raw(`
		SELECT * FROM bill_heads
		WHERE id = $1 AND user_id = $2
		LIMIT 1
	`, billId, userId).Scan(&bill).Error
	if err != nil {
		return nil, err
	}

	// Preload Lines and Matches manually
	var lines []model.BillLine
	err = r.db.Raw(`
		SELECT * FROM bill_lines
		WHERE bill_id = $1
	`, bill.Id).Scan(&lines).Error
	if err == nil {
		for _, line := range lines {
			var match model.Match
			_ = r.db.Raw(`SELECT * FROM matches WHERE id = $1`, line.MatchId).Scan(&match).Error
			line.Match = match
		}
		bill.Lines = lines
	}

	return &bill, nil
}

// GetAll retrieves all bills for a specific user
func (r *billRepositoryImpl) GetAll(userId string) ([]*model.BillHead, error) {
	var bills []*model.BillHead

	err := r.db.Raw(`
		SELECT * FROM bill_heads
		WHERE user_id = $1
		ORDER BY created_at DESC
	`, userId).Scan(&bills).Error
	if err != nil {
		return nil, err
	}

	// Preload Lines & Matches for each bill
	for _, bill := range bills {
		var lines []model.BillLine
		_ = r.db.Raw(`
			SELECT * FROM bill_lines
			WHERE bill_id = $1
		`, bill.Id).Scan(&lines).Error

		for _, line := range lines {
			var match model.Match
			_ = r.db.Raw(`SELECT * FROM matches WHERE id = $1`, line.MatchId).Scan(&match).Error
			line.Match = match
		}
		bill.Lines = lines
	}

	return bills, nil
}

// GetAllAdmin retrieves all bills (with users)
func (r *billRepositoryImpl) GetAllAdmin() ([]*model.BillHead, error) {
	var bills []*model.BillHead

	err := r.db.Raw(`
		SELECT * FROM bill_heads
		ORDER BY created_at DESC
	`).Scan(&bills).Error
	if err != nil {
		return nil, err
	}

	// Preload Lines, Matches, and Users manually
	for _, bill := range bills {
		var lines []model.BillLine
		_ = r.db.Raw(`
			SELECT * FROM bill_lines
			WHERE bill_id = $1
		`, bill.Id).Scan(&lines).Error

		for _, line := range lines {
			var match model.Match
			_ = r.db.Raw(`SELECT * FROM matches WHERE id = $1`, line.MatchId).Scan(&match).Error
			line.Match = match
		}

		bill.Lines = lines

		var user model.User
		_ = r.db.Raw(`SELECT * FROM users WHERE id = $1`, bill.UserId).Scan(&user).Error
		bill.User = user
	}

	return bills, nil
}

// Update an existing bill
func (r *billRepositoryImpl) Update(bill *model.BillHead) error {
	return r.db.Exec(`
		UPDATE bill_heads
		SET total = $1,
		    updated_at = NOW()
		WHERE id = $2
	`, bill.Total, bill.Id).Error
}

// Delete a bill by its ID
func (r *billRepositoryImpl) Delete(id string) error {
	return r.db.Exec(`
		DELETE FROM bill_heads
		WHERE id = $1
	`, id).Error
}
