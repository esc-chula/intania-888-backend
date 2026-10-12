package policy

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

// GORMRepository stores access policies using the established PostgreSQL schema.
type GORMRepository struct {
	db *gorm.DB
}

// NewGORMRepository constructs the PostgreSQL policy adapter.
func NewGORMRepository(db *gorm.DB) *GORMRepository {
	return &GORMRepository{db: db}
}

// ListActive loads enabled policies that have not expired.
func (r *GORMRepository) ListActive(ctx context.Context, now time.Time) ([]*AccessPolicy, error) {
	var policies []*accessPolicyRow
	err := r.db.WithContext(ctx).
		Where("enabled = ?", true).
		Where("expires_at IS NULL OR expires_at > ?", now).
		Order("created_at DESC, id DESC").
		Find(&policies).Error

	return policiesFromRows(policies), err
}

// List loads a stable ordered policy page.
func (r *GORMRepository) List(ctx context.Context, filter ListFilter) (ListResult, error) {
	query := r.db.WithContext(ctx).Model(&accessPolicyRow{}).Order("created_at DESC, id DESC")
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
	var policies []*accessPolicyRow
	if err := query.Offset(filter.Offset).Limit(limit + 1).Find(&policies).Error; err != nil {
		return ListResult{}, err
	}
	result := ListResult{Items: policiesFromRows(policies)}
	if len(policies) > limit {
		result.HasMore = true
		result.Items = result.Items[:limit]
	}

	return result, nil
}

// FindByID loads a policy and translates a missing row.
func (r *GORMRepository) FindByID(ctx context.Context, id string) (*AccessPolicy, error) {
	var policy accessPolicyRow
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&policy).Error; err != nil {
		return nil, translateStorageError(err)
	}

	return policyFromRow(&policy), nil
}

// FindByIdentity loads the unique principal identity.
func (r *GORMRepository) FindByIdentity(ctx context.Context, kind, principalType, principal string) (*AccessPolicy, error) {
	var policy accessPolicyRow
	if err := r.db.WithContext(ctx).Where("kind = ? AND principal_type = ? AND principal = ?", kind, principalType, principal).First(&policy).Error; err != nil {
		return nil, translateStorageError(err)
	}

	return policyFromRow(&policy), nil
}

// Create inserts a policy and translates unique-key conflicts.
func (r *GORMRepository) Create(ctx context.Context, policy *AccessPolicy) error {
	row := policyToRow(policy)
	if err := r.db.WithContext(ctx).Create(row).Error; err != nil {
		return translateStorageError(err)
	}
	*policy = *policyFromRow(row)

	return nil
}

// Update persists the mutable fields of a policy.
func (r *GORMRepository) Update(ctx context.Context, policy *AccessPolicy) error {
	return translateStorageError(r.db.WithContext(ctx).Model(&accessPolicyRow{}).Where("id = ?", policy.ID).Updates(map[string]interface{}{
		"reason":     policy.Reason,
		"enabled":    policy.Enabled,
		"expires_at": policy.ExpiresAt,
		"updated_at": policy.UpdatedAt,
	}).Error)
}

// accessPolicyRow owns the ORM schema independently from service snapshots.
type accessPolicyRow struct {
	ID            string     `gorm:"primaryKey;type:varchar(100)"`
	Kind          string     `gorm:"type:varchar(20);not null"`
	PrincipalType string     `gorm:"column:principal_type;type:varchar(20);not null"`
	Principal     string     `gorm:"type:varchar(320);not null"`
	Reason        string     `gorm:"type:varchar(500);not null"`
	Enabled       bool       `gorm:"not null;default:true"`
	ExpiresAt     *time.Time `gorm:"column:expires_at"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// TableName preserves the established access-policy table.
func (accessPolicyRow) TableName() string {
	return "auth_access_policies"
}

func policyToRow(policy *AccessPolicy) *accessPolicyRow {
	return &accessPolicyRow{
		ID:            policy.ID,
		Kind:          policy.Kind,
		PrincipalType: policy.PrincipalType,
		Principal:     policy.Principal,
		Reason:        policy.Reason,
		Enabled:       policy.Enabled,
		ExpiresAt:     policy.ExpiresAt,
		CreatedAt:     policy.CreatedAt,
		UpdatedAt:     policy.UpdatedAt,
	}
}

func policyFromRow(row *accessPolicyRow) *AccessPolicy {
	if row == nil {
		return nil
	}

	return &AccessPolicy{
		ID:            row.ID,
		Kind:          row.Kind,
		PrincipalType: row.PrincipalType,
		Principal:     row.Principal,
		Reason:        row.Reason,
		Enabled:       row.Enabled,
		ExpiresAt:     row.ExpiresAt,
		CreatedAt:     row.CreatedAt,
		UpdatedAt:     row.UpdatedAt,
	}
}

func policiesFromRows(rows []*accessPolicyRow) []*AccessPolicy {
	if rows == nil {
		return nil
	}
	policies := make([]*AccessPolicy, len(rows))
	for i, row := range rows {
		policies[i] = policyFromRow(row)
	}

	return policies
}

func translateStorageError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrPolicyNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrPolicyConflict
	}

	return err
}
