// package match

// import (
// 	"time"

// 	"github.com/esc-chula/intania-888-backend/internal/model"
// 	"gorm.io/gorm"
// )

// type matchRepositoryImpl struct {
// 	db *gorm.DB
// }

// func NewMatchRepository(db *gorm.DB) MatchRepository {
// 	return &matchRepositoryImpl{db}
// }

// func (r *matchRepositoryImpl) Create(match *model.Match) error {

// 	return r.db.Create(match).Error
// }

// func (r *matchRepositoryImpl) GetById(matchId string) (*model.Match, error) {
// 	var match model.Match
// 	err := r.db.Where("id = ?", matchId).First(&match).Error
// 	if err != nil {
// 		return nil, err
// 	}
// 	return &match, nil
// }

// func (r *matchRepositoryImpl) GetAll(filter *model.MatchFilter) ([]*model.Match, error) {
// 	var matches []*model.Match
// 	db := r.db

// 	if filter != nil {
// 		if filter.TypeId != "" {
// 			db = db.Where("type_id = ?", filter.TypeId)
// 		}

// 		now := time.Now()

// 		switch filter.Schedule {
// 		case model.Schedule:
// 			db = db.Where("end_time > ?", now)
// 		case model.Result:
// 			db = db.Where("end_time <= ?", now)
// 		}
// 	}

// 	err := db.Order("start_time").Find(&matches).Error
// 	if err != nil {
// 		return nil, err
// 	}
// 	return matches, nil
// }

// func (r *matchRepositoryImpl) CountBetsForTeam(matchId string, teamId string) (int64, error) {
// 	var count int64
// 	err := r.db.Model(&model.BillLine{}).Where("match_id = ? AND betting_on = ?", matchId, teamId).Count(&count).Error
// 	if err != nil {
// 		return 0, err
// 	}
// 	return count, nil
// }

// func (r *matchRepositoryImpl) UpdateScore(match *model.Match) error {
// 	return r.db.Model(&model.Match{}).
// 		Where("id = ?", match.Id).
// 		Updates(map[string]interface{}{
// 			"teama_score": match.TeamA_Score,
// 			"teamb_score": match.TeamB_Score,
// 		}).Error
// }

// func (r *matchRepositoryImpl) UpdateWinner(match *model.Match) error {
// 	return r.db.Model(&model.Match{}).
// 		Where("id = ?", match.Id).
// 		Update("winner_id", match.WinnerId).Error
// }

// func (r *matchRepositoryImpl) Delete(id string) error {
// 	return r.db.Delete(&model.Match{}, "id = ?", id).Error
// }

// func (r *matchRepositoryImpl) GetBillHeadsForMatch(matchId string) ([]*model.BillHead, error) {
// 	var billHeads []*model.BillHead
// 	err := r.db.Table("bill_heads").Preload("Lines").Preload("Lines.Match").
// 		Joins("JOIN bill_lines ON bill_heads.id = bill_lines.bill_id").
// 		Where("bill_lines.match_id = ?", matchId).
// 		Find(&billHeads).Error
// 	if err != nil {
// 		return nil, err
// 	}
// 	return billHeads, nil
// }

// func (r *matchRepositoryImpl) PayoutToUser(userId string, amount float64) error {
// 	return r.db.Model(&model.User{}).Where("id = ?", userId).
// 		Update("remaining_coin", gorm.Expr("remaining_coin + ?", amount)).Error
// }

// func (r *matchRepositoryImpl) MarkBillLineAsPaid(billId string, matchId string) error {
// 	return r.db.Model(&model.BillLine{}).
// 		Where("bill_id = ? AND match_id = ?", billId, matchId).
// 		Update("is_paid", true).Error
// }

// func (r *matchRepositoryImpl) UpdateMatch(match *model.Match) error {
// 	return r.db.Model(&model.Match{}).
// 		Where("id = ?", match.Id).
// 		Updates(map[string]interface{}{
// 			"is_draw":    match.IsDraw,
// 			"updated_at": time.Now(),
// 		}).Error
// }

package match

import (
	"fmt"
	"time"

	"github.com/esc-chula/intania-888-backend/internal/model"
	"gorm.io/gorm"
)

type matchRepositoryImpl struct {
	db *gorm.DB
}

func NewMatchRepository(db *gorm.DB) MatchRepository {
	return &matchRepositoryImpl{db}
}

func (r *matchRepositoryImpl) Create(match *model.Match) error {
	return r.db.Create(match).Error
}

func (r *matchRepositoryImpl) GetById(matchId string) (*model.Match, error) {
	var match model.Match
	err := r.db.Raw(`
		SELECT * FROM matches
		WHERE id = $1
		LIMIT 1
	`, matchId).Scan(&match).Error
	if err != nil {
		return nil, err
	}

	if match.Id == "" {
		return nil, gorm.ErrRecordNotFound
	}

	return &match, nil
}

func (r *matchRepositoryImpl) GetAll(filter *model.MatchFilter) ([]*model.Match, error) {
	var matches []*model.Match

	baseQuery := `
		SELECT * FROM matches
	`
	whereClauses := []string{}
	args := []interface{}{}
	argIndex := 1

	if filter != nil {
		if filter.TypeId != "" {
			whereClauses = append(whereClauses, `type_id = $`+itoa(argIndex))
			args = append(args, filter.TypeId)
			argIndex++
		}

		now := time.Now()
		switch filter.Schedule {
		case model.Schedule:
			whereClauses = append(whereClauses, `end_time > $`+itoa(argIndex))
			args = append(args, now)
			argIndex++
		case model.Result:
			whereClauses = append(whereClauses, `end_time <= $`+itoa(argIndex))
			args = append(args, now)
			argIndex++
		}
	}

	if len(whereClauses) > 0 {
		baseQuery += " WHERE " + joinClauses(whereClauses, " AND ")
	}

	baseQuery += " ORDER BY start_time"

	err := r.db.Raw(baseQuery, args...).Scan(&matches).Error
	if err != nil {
		return nil, err
	}
	return matches, nil
}

func (r *matchRepositoryImpl) CountBetsForTeam(matchId string, teamId string) (int64, error) {
	var count int64
	err := r.db.Raw(`
		SELECT COUNT(*) FROM bill_lines
		WHERE match_id = $1 AND betting_on = $2
	`, matchId, teamId).Scan(&count).Error
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (r *matchRepositoryImpl) UpdateScore(match *model.Match) error {
	return r.db.Exec(`
		UPDATE matches
		SET teama_score = $1, teamb_score = $2
		WHERE id = $3
	`, match.TeamA_Score, match.TeamB_Score, match.Id).Error
}

func (r *matchRepositoryImpl) UpdateWinner(match *model.Match) error {
	return r.db.Exec(`
		UPDATE matches
		SET winner_id = $1
		WHERE id = $2
	`, match.WinnerId, match.Id).Error
}

func (r *matchRepositoryImpl) Delete(id string) error {
	return r.db.Exec(`
		DELETE FROM matches
		WHERE id = $1
	`, id).Error
}

func (r *matchRepositoryImpl) GetBillHeadsForMatch(matchId string) ([]*model.BillHead, error) {
	var billHeads []*model.BillHead
	err := r.db.Raw(`
		SELECT bh.*
		FROM bill_heads bh
		JOIN bill_lines bl ON bh.id = bl.bill_id
		WHERE bl.match_id = $1
	`, matchId).Scan(&billHeads).Error
	if err != nil {
		return nil, err
	}

	// Optional: preload lines if needed
	for _, bh := range billHeads {
		var lines []model.BillLine
		_ = r.db.Raw(`
			SELECT * FROM bill_lines
			WHERE bill_id = $1
		`, bh.Id).Scan(&lines).Error
		bh.Lines = lines
	}
	return billHeads, nil
}

func (r *matchRepositoryImpl) PayoutToUser(userId string, amount float64) error {
	return r.db.Exec(`
		UPDATE users
		SET remaining_coin = remaining_coin + $1
		WHERE id = $2
	`, amount, userId).Error
}

func (r *matchRepositoryImpl) MarkBillLineAsPaid(billId string, matchId string) error {
	return r.db.Exec(`
		UPDATE bill_lines
		SET is_paid = TRUE
		WHERE bill_id = $1 AND match_id = $2
	`, billId, matchId).Error
}

func (r *matchRepositoryImpl) UpdateMatch(match *model.Match) error {
	return r.db.Exec(`
		UPDATE matches
		SET is_draw = $1, updated_at = $2
		WHERE id = $3
	`, match.IsDraw, time.Now(), match.Id).Error
}

// --- Helper functions ---

func itoa(i int) string {
	return fmt.Sprintf("%d", i)
}

func joinClauses(clauses []string, sep string) string {
	out := ""
	for i, c := range clauses {
		if i > 0 {
			out += sep
		}
		out += c
	}
	return out
}
