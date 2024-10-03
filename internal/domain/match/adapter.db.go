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
			"teama_score": match.TeamA_Score,
			"teamb_score": match.TeamB_Score,
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
