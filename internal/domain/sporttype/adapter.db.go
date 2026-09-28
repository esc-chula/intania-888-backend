package sporttype

import (
	"context"

	"gorm.io/gorm"

	persistence "github.com/esc-chula/intania-888-backend/internal/persistence/model"
)

// GORMRepository loads neutral sport catalogue entries from PostgreSQL.
type GORMRepository struct {
	db *gorm.DB
}

// NewGORMRepository constructs the sport catalogue query adapter.
func NewGORMRepository(db *gorm.DB) *GORMRepository {
	return &GORMRepository{
		db: db,
	}
}

// GetAllSportTypes loads every sport entry and maps persistence rows to catalogue values.
func (r *GORMRepository) GetAllSportTypes(ctx context.Context) ([]*SportType, error) {
	var sportTypes []*persistence.SportType

	if err := r.db.WithContext(ctx).Find(&sportTypes).Error; err != nil {
		return nil, err
	}

	rows := make([]*SportType, len(sportTypes))
	for i, row := range sportTypes {
		rows[i] = &SportType{ID: row.ID, Title: row.Title}
	}
	return rows, nil
}
