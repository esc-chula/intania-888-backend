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

	// Base query
	query := r.db.Table("colors").
		Select(`
			colors.*, 
			COUNT(matches.id) as total_matches, 
			SUM(CASE WHEN matches.is_draw = TRUE THEN 1 ELSE 0 END) as drawn, 
			SUM(CASE WHEN matches.winner_id = colors.id THEN 1 ELSE 0 END) as won
		`).
		Group("colors.id")

	// Join
	matchJoin := `
		LEFT JOIN matches 
		ON (matches.winner_id IS NOT NULL OR matches.is_draw = TRUE) 
		AND (colors.id = matches.teama_id OR colors.id = matches.teamb_id)
	`
	if typeId != "" {
		matchJoin += " AND matches.type_id = ?"
		query = query.Joins(matchJoin, typeId)
	} else {
		query = query.Joins(matchJoin)
	}

	// Execute
	if err := query.Find(&colors).Error; err != nil {
		return nil, err
	}

	return colors, nil
}
