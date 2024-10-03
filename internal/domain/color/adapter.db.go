package color

import (
	"github.com/esc-chula/intania-888-backend/internal/model"
	"gorm.io/gorm"
)

type colorRepository struct {
	db *gorm.DB
}

func NewColorRepository(db *gorm.DB) ColorRepository {
	return &colorRepository{
		db: db,
	}
}

func (r *colorRepository) GetAllColors(typeId string) ([]*model.Color, error) {
	var colors []*model.Color
	query := r.db.Preload("Won").
		Table("colors").
		Select("colors.*, COUNT(matches.id) as total_matches").
		Joins("LEFT JOIN matches ON matches.teama_id = colors.id OR matches.teamb_id = colors.id").
		Group("colors.id")

	if typeId != "" {
		query = query.Where("matches.type_id = ?", typeId)
	}

	if err := query.Find(&colors).Error; err != nil {
		return nil, err
	}

	return colors, nil
}
