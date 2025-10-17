// package sporttype

// import (
// 	"github.com/esc-chula/intania-888-backend/internal/model"
// 	"gorm.io/gorm"
// )

// type sportTypeRepository struct {
// 	db *gorm.DB
// }

// func NewSportTypeRepository(db *gorm.DB) SportTypeRepository {
// 	return &sportTypeRepository{
// 		db: db,
// 	}
// }

// func (r *sportTypeRepository) GetAllSportTypes() ([]*model.SportType, error) {
// 	var sportTypes []*model.SportType

// 	if err := r.db.Find(&sportTypes).Error; err != nil {
// 		return nil, err
// 	}

// 	return sportTypes, nil
// }

package sporttype

import (
	"github.com/esc-chula/intania-888-backend/internal/model"
	"gorm.io/gorm"
)

type sportTypeRepository struct {
	db *gorm.DB
}

func NewSportTypeRepository(db *gorm.DB) SportTypeRepository {
	return &sportTypeRepository{db: db}
}

// GetAllSportTypes retrieves all sport types (PostgreSQL style)
func (r *sportTypeRepository) GetAllSportTypes() ([]*model.SportType, error) {
	var sportTypes []*model.SportType

	err := r.db.Raw(`
		SELECT * FROM sport_types
		ORDER BY id ASC
	`).Scan(&sportTypes).Error
	if err != nil {
		return nil, err
	}

	return sportTypes, nil
}
