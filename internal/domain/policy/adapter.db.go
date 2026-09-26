package policy

import (
	"time"

	"gorm.io/gorm"
)

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) ListActive(now time.Time) ([]*AccessPolicy, error) {
	var policies []*AccessPolicy
	err := r.db.
		Where("enabled = ?", true).
		Where("expires_at IS NULL OR expires_at > ?", now).
		Order("created_at DESC, id DESC").
		Find(&policies).Error
	return policies, err
}

func (r *repository) List(filter ListFilter) (ListResult, error) {
	query := r.db.Model(&AccessPolicy{}).Order("created_at DESC, id DESC")
	if filter.Kind != "" {
		query = query.Where("kind = ?", filter.Kind)
	}
	if filter.PrincipalType != "" {
		query = query.Where("principal_type = ?", filter.PrincipalType)
	}
	switch filter.Status {
	case StatusActive:
		query = query.Where("enabled = ?", true).Where("expires_at IS NULL OR expires_at > ?", time.Now())
	case StatusInactive:
		query = query.Where("(enabled = ? OR expires_at <= ?)", false, time.Now())
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	var policies []*AccessPolicy
	if err := query.Offset(filter.Offset).Limit(limit + 1).Find(&policies).Error; err != nil {
		return ListResult{}, err
	}
	result := ListResult{Items: policies}
	if len(policies) > limit {
		result.HasMore = true
		result.Items = policies[:limit]
	}
	return result, nil
}

func (r *repository) FindByID(id string) (*AccessPolicy, error) {
	var policy AccessPolicy
	if err := r.db.Where("id = ?", id).First(&policy).Error; err != nil {
		return nil, err
	}
	return &policy, nil
}

func (r *repository) FindByIdentity(kind, principalType, principal string) (*AccessPolicy, error) {
	var policy AccessPolicy
	if err := r.db.Where("kind = ? AND principal_type = ? AND principal = ?", kind, principalType, principal).First(&policy).Error; err != nil {
		return nil, err
	}
	return &policy, nil
}

func (r *repository) Create(policy *AccessPolicy) error {
	return r.db.Create(policy).Error
}

func (r *repository) Update(policy *AccessPolicy) error {
	return r.db.Model(&AccessPolicy{}).Where("id = ?", policy.ID).Updates(map[string]interface{}{
		"reason":     policy.Reason,
		"enabled":    policy.Enabled,
		"expires_at": policy.ExpiresAt,
		"updated_at": policy.UpdatedAt,
	}).Error
}
