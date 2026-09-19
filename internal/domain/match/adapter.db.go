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
	return &matchRepositoryImpl{db: db}
}

func (r *matchRepositoryImpl) Create(v *model.Match) error {
	return r.db.Create(v).Error
}

func (r *matchRepositoryImpl) GetById(id string) (*model.Match, error) {
	var v model.Match

	if e := r.db.First(&v, "id = ?", id).Error; e != nil {
		return nil, e
	}

	return &v, nil
}

func (r *matchRepositoryImpl) GetAll(f *model.MatchFilter) ([]*model.Match, error) {
	var v []*model.Match

	q := r.db

	if f != nil {
		if f.TypeId != "" {
			q = q.Where("type_id = ?", f.TypeId)
		}

		switch f.Schedule {
		case model.Schedule:
			q = q.Where("end_time > ?", time.Now())
		case model.Result:
			q = q.Where("end_time <= ?", time.Now())
		}
	}

	e := q.
		Order("start_time").
		Find(&v).
		Error

	return v, e
}

func (r *matchRepositoryImpl) CountBetsForTeam(mid, team string) (int64, error) {
	var n int64

	e := r.db.
		Table("bill_lines").
		Joins("JOIN bill_heads ON bill_heads.id = bill_lines.bill_id").
		Where("bill_lines.match_id = ? AND bill_lines.betting_on = ? AND bill_heads.status = 'PENDING'", mid, team).
		Count(&n).
		Error

	return n, e
}

func (r *matchRepositoryImpl) UpdateScore(v *model.Match) error {
	updates := map[string]any{
		"teama_score": v.TeamA_Score,
		"teamb_score": v.TeamB_Score,
	}

	return r.db.Model(v).Updates(updates).Error
}

func (r *matchRepositoryImpl) UpdateMatch(v *model.Match) error {
	updates := map[string]any{
		"teama_id":   v.TeamA_Id,
		"teamb_id":   v.TeamB_Id,
		"type_id":    v.TypeId,
		"start_time": v.StartTime,
		"end_time":   v.EndTime,
		"updated_at": time.Now(),
	}

	return r.db.Model(v).Updates(updates).Error
}

func (r *matchRepositoryImpl) Delete(id string) error {
	return r.db.Delete(&model.Match{}, "id = ?", id).Error
}
