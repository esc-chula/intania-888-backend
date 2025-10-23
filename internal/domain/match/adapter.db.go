package match

import (
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
	err := r.db.Where("id = ?", matchId).First(&match).Error
	if err != nil {
		return nil, err
	}
	return &match, nil
}

func (r *matchRepositoryImpl) GetAll(filter *model.MatchFilter) ([]*model.Match, error) {
	var matches []*model.Match
	db := r.db

	if filter != nil {
		if filter.TypeId != "" {
			db = db.Where("type_id = ?", filter.TypeId)
		}

		now := time.Now()

		switch filter.Schedule {
		case model.Schedule:
			db = db.Where("end_time > ?", now)
		case model.Result:
			db = db.Where("end_time <= ?", now)
		}
	}

	err := db.Order("start_time").Find(&matches).Error
	if err != nil {
		return nil, err
	}
	return matches, nil
}

func (r *matchRepositoryImpl) CountBetsForTeam(matchId string, teamId string) (int64, error) {
	var count int64
	err := r.db.Model(&model.BillLine{}).Where("match_id = ? AND betting_on = ?", matchId, teamId).Count(&count).Error
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (r *matchRepositoryImpl) UpdateScore(match *model.Match) error {
	return r.db.Model(&model.Match{}).
		Where("id = ?", match.Id).
		Updates(map[string]interface{}{
			"team_a_score": match.TeamA_Score,
			"team_b_score": match.TeamB_Score,
		}).Error
}

func (r *matchRepositoryImpl) UpdateWinner(match *model.Match) error {
	return r.db.Model(&model.Match{}).
		Where("id = ?", match.Id).
		Update("winner_id", match.WinnerId).Error
}

func (r *matchRepositoryImpl) Delete(id string) error {
	return r.db.Delete(&model.Match{}, "id = ?", id).Error
}

func (r *matchRepositoryImpl) GetBillHeadsForMatch(matchId string) ([]*model.BillHead, error) {
	var billHeads []*model.BillHead
	err := r.db.Table("bill_heads").Preload("Lines").Preload("Lines.Match").
		Joins("JOIN bill_lines ON bill_heads.id = bill_lines.bill_id").
		Where("bill_lines.match_id = ?", matchId).
		Find(&billHeads).Error
	if err != nil {
		return nil, err
	}
	return billHeads, nil
}

func (r *matchRepositoryImpl) PayoutToUser(userId string, amount float64) error {
	return r.db.Model(&model.User{}).Where("id = ?", userId).
		Update("remaining_coin", gorm.Expr("remaining_coin + ?", amount)).Error
}

func (r *matchRepositoryImpl) MarkBillLineAsPaid(billId string, matchId string) error {
	return r.db.Model(&model.BillLine{}).
		Where("bill_id = ? AND match_id = ?", billId, matchId).
		Update("is_paid", true).Error
}

func (r *matchRepositoryImpl) UpdateMatch(match *model.Match) error {
	return r.db.Model(&model.Match{}).
		Where("id = ?", match.Id).
		Updates(map[string]interface{}{
			"team_a_id":  match.TeamA_Id,
			"team_b_id":  match.TeamB_Id,
			"type_id":    match.TypeId,
			"start_time": match.StartTime,
			"end_time":   match.EndTime,
			"is_draw":    match.IsDraw,
			"updated_at": time.Now(),
		}).Error
}
