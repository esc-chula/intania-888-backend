package color

import (
	"context"

	"gorm.io/gorm"

	persistence "github.com/esc-chula/intania-888-backend/internal/persistence/model"
)

// GORMRepository queries completed-match aggregates without applying response formatting.
type GORMRepository struct {
	db *gorm.DB
}

// NewGORMRepository constructs the color aggregate-query adapter.
func NewGORMRepository(db *gorm.DB) *GORMRepository {
	return &GORMRepository{
		db: db,
	}
}

// GetAllLeaderboards aggregates terminal matches for each color; an empty type ID includes every sport.
func (r *GORMRepository) GetAllLeaderboards(ctx context.Context, typeID string) ([]*Standing, error) {
	var colors []*persistence.Color

	// Base query
	query := r.db.WithContext(ctx).Table("colors").
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
	if typeID != "" {
		matchJoin += " AND matches.type_id = ?"
		query = query.Joins(matchJoin, typeID)
	} else {
		query = query.Joins(matchJoin)
	}

	// Execute
	if err := query.Find(&colors).Error; err != nil {
		return nil, err
	}

	rows := make([]*Standing, len(colors))
	for i, color := range colors {
		rows[i] = &Standing{
			ID:           color.ID,
			Title:        color.Title,
			Won:          int64(color.Won),
			Drawn:        int64(color.Drawn),
			TotalMatches: int64(color.TotalMatches),
		}
	}
	return rows, nil
}

// GetGroupStageTable aggregates distinct terminal matches with optional sport and group-stage filters.
func (r *GORMRepository) GetGroupStageTable(ctx context.Context, typeID, groupID string) ([]*Standing, error) {
	var colors []*persistence.Color

	// Base query
	query := r.db.WithContext(ctx).Table("colors").
		Select(`
			colors.*, 
			COUNT(DISTINCT matches.id) as total_matches, 
			COUNT(DISTINCT CASE WHEN matches.is_draw = TRUE THEN matches.id END) as drawn, 
			COUNT(DISTINCT CASE WHEN matches.winner_id = colors.id THEN matches.id END) as won
		`).
		Group("colors.id")

	// Join matches table
	matchJoin := `
		LEFT JOIN matches 
		ON (matches.winner_id IS NOT NULL OR matches.is_draw = TRUE) 
		AND (colors.id = matches.teama_id OR colors.id = matches.teamb_id)
	`
	if typeID != "" {
		matchJoin += " AND matches.type_id = ?"
		query = query.Joins(matchJoin, typeID)
	} else {
		query = query.Joins(matchJoin)
	}

	// Join group_stages table to filter by groupID
	if groupID != "" {
		query = query.Joins(`
			INNER JOIN group_stages
			ON group_stages.color_id = colors.id 
			AND group_stages.id = ?`, groupID)
	}

	// Execute the query
	if err := query.Find(&colors).Error; err != nil {
		return nil, err
	}

	rows := make([]*Standing, len(colors))
	for i, color := range colors {
		rows[i] = &Standing{
			ID:           color.ID,
			Title:        color.Title,
			Won:          int64(color.Won),
			Drawn:        int64(color.Drawn),
			TotalMatches: int64(color.TotalMatches),
		}
	}
	return rows, nil
}
