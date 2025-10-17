// package color

// import (
// 	"github.com/esc-chula/intania-888-backend/internal/model"
// 	"gorm.io/gorm"
// )

// type colorRepository struct {
// 	db *gorm.DB
// }

// func NewColorRepository(db *gorm.DB) ColorRepository {
// 	return &colorRepository{
// 		db: db,
// 	}
// }

// func (r *colorRepository) GetAllLeaderboards(typeId string) ([]*model.Color, error) {
// 	var colors []*model.Color

// 	// Base query
// 	query := r.db.Table("colors").
// 		Select(`
// 			colors.*,
// 			COUNT(matches.id) as total_matches,
// 			SUM(CASE WHEN matches.is_draw = TRUE THEN 1 ELSE 0 END) as drawn,
// 			SUM(CASE WHEN matches.winner_id = colors.id THEN 1 ELSE 0 END) as won
// 		`).
// 		Group("colors.id")

// 	// Join
// 	matchJoin := `
// 		LEFT JOIN matches
// 		ON (matches.winner_id IS NOT NULL OR matches.is_draw = TRUE)
// 		AND (colors.id = matches.teama_id OR colors.id = matches.teamb_id)
// 	`
// 	if typeId != "" {
// 		matchJoin += " AND matches.type_id = ?"
// 		query = query.Joins(matchJoin, typeId)
// 	} else {
// 		query = query.Joins(matchJoin)
// 	}

// 	// Execute
// 	if err := query.Find(&colors).Error; err != nil {
// 		return nil, err
// 	}

// 	return colors, nil
// }

// func (r *colorRepository) GetGroupStageTable(typeId, groupId string) ([]*model.Color, error) {
// 	var colors []*model.Color

// 	// Base query
// 	query := r.db.Table("colors").
// 		Select(`
// 			colors.*,
// 			COUNT(DISTINCT matches.id) as total_matches,
// 			COUNT(DISTINCT CASE WHEN matches.is_draw = TRUE THEN matches.id END) as drawn,
// 			COUNT(DISTINCT CASE WHEN matches.winner_id = colors.id THEN matches.id END) as won
// 		`).
// 		Group("colors.id")

// 	// Join matches table
// 	matchJoin := `
// 		LEFT JOIN matches
// 		ON (matches.winner_id IS NOT NULL OR matches.is_draw = TRUE)
// 		AND (colors.id = matches.teama_id OR colors.id = matches.teamb_id)
// 	`
// 	if typeId != "" {
// 		matchJoin += " AND matches.type_id = ?"
// 		query = query.Joins(matchJoin, typeId)
// 	} else {
// 		query = query.Joins(matchJoin)
// 	}

// 	// Join group_stages table to filter by groupId
// 	if groupId != "" {
// 		query = query.Joins(`
// 			INNER JOIN group_stages
// 			ON group_stages.color_id = colors.id
// 			AND group_stages.id = ?`, groupId)
// 	}

// 	// Execute the query
// 	if err := query.Find(&colors).Error; err != nil {
// 		return nil, err
// 	}

// 	return colors, nil
// }

package color

import (
	"fmt"

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

// GetAllLeaderboards retrieves leaderboard info for each color
func (r *colorRepository) GetAllLeaderboards(typeId string) ([]*model.Color, error) {
	var colors []*model.Color

	baseSQL := `
		SELECT
			colors.*,
			COUNT(matches.id) AS total_matches,
			SUM(CASE WHEN matches.is_draw = TRUE THEN 1 ELSE 0 END) AS drawn,
			SUM(CASE WHEN matches.winner_id = colors.id THEN 1 ELSE 0 END) AS won
		FROM colors
		LEFT JOIN matches
			ON (matches.winner_id IS NOT NULL OR matches.is_draw = TRUE)
			AND (colors.id = matches.teama_id OR colors.id = matches.teamb_id)
	`
	args := []interface{}{}

	if typeId != "" {
		baseSQL += " AND matches.type_id = $1"
		args = append(args, typeId)
	}

	baseSQL += " GROUP BY colors.id"

	err := r.db.Raw(baseSQL, args...).Scan(&colors).Error
	if err != nil {
		return nil, err
	}

	return colors, nil
}

// GetGroupStageTable retrieves leaderboard info for a specific group stage
func (r *colorRepository) GetGroupStageTable(typeId, groupId string) ([]*model.Color, error) {
	var colors []*model.Color

	baseSQL := `
		SELECT
			colors.*,
			COUNT(DISTINCT matches.id) AS total_matches,
			COUNT(DISTINCT CASE WHEN matches.is_draw = TRUE THEN matches.id END) AS drawn,
			COUNT(DISTINCT CASE WHEN matches.winner_id = colors.id THEN matches.id END) AS won
		FROM colors
		LEFT JOIN matches
			ON (matches.winner_id IS NOT NULL OR matches.is_draw = TRUE)
			AND (colors.id = matches.teama_id OR colors.id = matches.teamb_id)
	`
	args := []interface{}{}
	argIndex := 1

	if typeId != "" {
		baseSQL += fmt.Sprintf(" AND matches.type_id = $%d", argIndex)
		args = append(args, typeId)
		argIndex++
	}

	if groupId != "" {
		baseSQL += fmt.Sprintf(`
			INNER JOIN group_stages
				ON group_stages.color_id = colors.id
				AND group_stages.id = $%d
		`, argIndex)
		args = append(args, groupId)
		argIndex++
	}

	baseSQL += " GROUP BY colors.id"

	err := r.db.Raw(baseSQL, args...).Scan(&colors).Error
	if err != nil {
		return nil, err
	}

	return colors, nil
}
