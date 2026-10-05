package color

import (
	"context"

	"gorm.io/gorm"

	persistence "github.com/esc-chula/intania-888-backend/internal/persistence/model"
	"github.com/esc-chula/intania-888-backend/internal/value"
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

// CoinStandings sums USER coins per color across the color's groups, including
// colors with no members.
func (r *GORMRepository) CoinStandings(ctx context.Context) ([]*CoinStanding, error) {
	var rows []struct {
		ID          string
		Title       string
		TotalCoin   int64
		MemberCount int64
	}
	err := r.db.WithContext(ctx).Raw(`
		SELECT c.id AS id, c.title AS title,
			COALESCE(SUM(u.remaining_coin), 0)::bigint AS total_coin,
			COUNT(u.id) AS member_count
		FROM colors c
		LEFT JOIN intania_groups g ON g.color_id = c.id
		LEFT JOIN users u ON u.group_id = g.id AND u.role_id = 'USER'
		GROUP BY c.id, c.title
	`).Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	result := make([]*CoinStanding, len(rows))
	for i, row := range rows {
		coin, err := value.NewMoneyFromMinor(row.TotalCoin)
		if err != nil {
			return nil, err
		}
		result[i] = &CoinStanding{ID: row.ID, Title: row.Title, TotalCoin: coin, MemberCount: row.MemberCount}
	}

	return result, nil
}

// PredictionStandings counts correct and wrong bill lines per color. Only WON and
// LOST bills count, and only lines whose match has a winner: a draw line is
// skipped by settlement, so it is neither correct nor wrong here.
func (r *GORMRepository) PredictionStandings(ctx context.Context) ([]*PredictionStanding, error) {
	var rows []PredictionStanding
	err := r.db.WithContext(ctx).Raw(`
		SELECT c.id AS id, c.title AS title,
			COUNT(*) FILTER (WHERE m.winner_id = bl.betting_on) AS correct,
			COUNT(*) FILTER (WHERE m.winner_id IS NOT NULL AND m.winner_id <> bl.betting_on) AS wrong
		FROM colors c
		LEFT JOIN intania_groups g ON g.color_id = c.id
		LEFT JOIN users u ON u.group_id = g.id AND u.role_id = 'USER'
		LEFT JOIN bill_heads bh ON bh.user_id = u.id AND bh.status IN ('WON', 'LOST')
		LEFT JOIN bill_lines bl ON bl.bill_id = bh.id
		LEFT JOIN matches m ON m.id = bl.match_id
		GROUP BY c.id, c.title
	`).Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	result := make([]*PredictionStanding, len(rows))
	for i := range rows {
		result[i] = &rows[i]
	}

	return result, nil
}
