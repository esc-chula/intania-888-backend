package event

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/esc-chula/intania-888-backend/internal/identity"
	persistence "github.com/esc-chula/intania-888-backend/internal/persistence/model"
	"github.com/esc-chula/intania-888-backend/internal/value"
)

// GORMRepository executes event queries against PostgreSQL.
type GORMRepository struct{ db *gorm.DB }

type gormTransaction struct{ db *gorm.DB }

// NewGORMRepository constructs the event persistence adapter.
func NewGORMRepository(db *gorm.DB) *GORMRepository { return &GORMRepository{db: db} }

// WithinTransaction commits only when all service decisions and writes succeed.
func (r *GORMRepository) WithinTransaction(ctx context.Context, fn func(TransactionRepository) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { return fn(&gormTransaction{db: tx}) })
}

// ListRewards reads the configured overrides without applying reward rules.
func (r *GORMRepository) ListRewards(ctx context.Context) ([]DailyReward, error) {
	var rows []persistence.DailyReward
	if err := r.db.WithContext(ctx).Find(&rows).Error; err != nil {
		return nil, err
	}
	rewards := make([]DailyReward, len(rows))
	for i, row := range rows {
		rewards[i] = DailyReward{Date: row.Date, Reward: row.Reward}
	}
	return rewards, nil
}

// SetReward creates or replaces the override for one date.
func (r *GORMRepository) SetReward(ctx context.Context, reward DailyReward) error {
	return r.db.WithContext(ctx).Save(&persistence.DailyReward{Date: reward.Date, Reward: reward.Reward}).Error
}

// DeleteReward removes an override and translates a missing row.
func (r *GORMRepository) DeleteReward(ctx context.Context, date string) error {
	result := r.db.WithContext(ctx).Where("date = ?", date).Delete(&persistence.DailyReward{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrDailyRewardOverrideNotFound
	}
	return nil
}

// DeleteExpiredTokens deletes rows using the caller's cleanup instant.
func (r *GORMRepository) DeleteExpiredTokens(ctx context.Context, before time.Time) error {
	return r.db.WithContext(ctx).Where("expires_at < ?", before).Delete(&persistence.StealToken{}).Error
}

// GetUser reads a neutral account snapshot.
func (r *GORMRepository) GetUser(ctx context.Context, id string) (identity.User, error) {
	var row persistence.User
	if err := r.db.WithContext(ctx).First(&row, "id = ?", id).Error; err != nil {
		return identity.User{}, translateUserError(err)
	}
	return userFromRow(row), nil
}

// GetRandomEligibleUsers applies the service's eligibility threshold and existing database sampling.
func (r *GORMRepository) GetRandomEligibleUsers(ctx context.Context, excludeUserID string, minimumBalance int64, limit int) ([]identity.User, error) {
	var rows []persistence.User
	if err := r.db.WithContext(ctx).Where("id != ? AND remaining_coin >= ?", excludeUserID, minimumBalance).
		Order("RANDOM()").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	return usersFromRows(rows), nil
}

// GetReward reads an override and translates its absence.
func (r *gormTransaction) GetReward(ctx context.Context, date string) (DailyReward, error) {
	var row persistence.DailyReward
	if err := r.db.WithContext(ctx).Where("date = ?", date).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return DailyReward{}, ErrDailyRewardOverrideNotFound
		}
		return DailyReward{}, err
	}
	return DailyReward{Date: row.Date, Reward: row.Reward}, nil
}

// LockUser acquires one row lock at the service-selected point in the transaction.
func (r *gormTransaction) LockUser(ctx context.Context, id string) (identity.User, error) {
	var row persistence.User
	if err := r.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).First(&row).Error; err != nil {
		return identity.User{}, translateUserError(err)
	}
	return userFromRow(row), nil
}

// CreateDailyClaim inserts the idempotency record without replacing an existing claim.
func (r *gormTransaction) CreateDailyClaim(ctx context.Context, userID, date string, reward value.Money) (bool, error) {
	row := persistence.DailyRewardClaim{UserID: userID, Date: date, Reward: reward.MinorUnits()}
	result := r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&row)
	return result.RowsAffected == 1, result.Error
}

// SetUserBalance persists a service-calculated absolute balance.
func (r *gormTransaction) SetUserBalance(ctx context.Context, id string, balance value.Money) error {
	result := r.db.WithContext(ctx).Model(&persistence.User{}).Where("id = ?", id).Update("remaining_coin", balance.MinorUnits())
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrUserNotFound
	}
	return nil
}

// CreateStealToken stores the candidate IDs using the existing comma-separated representation.
func (r *gormTransaction) CreateStealToken(ctx context.Context, token StealToken) error {
	row := persistence.StealToken{ID: token.ID, UserID: token.UserID, Token: token.Token, IsUsed: token.IsUsed,
		AllowedVictimIDs: strings.Join(token.AllowedVictimIDs, ","), ExpiresAt: token.ExpiresAt,
		CreatedAt: token.CreatedAt, UpdatedAt: token.UpdatedAt}
	return r.db.WithContext(ctx).Create(&row).Error
}

// LockStealToken locks the token before the service checks ownership and expiry.
func (r *gormTransaction) LockStealToken(ctx context.Context, token string) (StealToken, error) {
	var row persistence.StealToken
	if err := r.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("token = ?", token).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return StealToken{}, ErrStealTokenInvalid
		}
		return StealToken{}, err
	}
	return StealToken{ID: row.ID, UserID: row.UserID, Token: row.Token, IsUsed: row.IsUsed,
		AllowedVictimIDs: splitCandidateIDs(row.AllowedVictimIDs), ExpiresAt: row.ExpiresAt,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}, nil
}

// GetUsersByIDs loads candidate state before the service applies its balance writes.
func (r *gormTransaction) GetUsersByIDs(ctx context.Context, ids []string) ([]identity.User, error) {
	var rows []persistence.User
	if err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&rows).Error; err != nil {
		return nil, err
	}
	return usersFromRows(rows), nil
}

// MarkTokenUsed conditionally consumes the token without overriding another consumption.
func (r *gormTransaction) MarkTokenUsed(ctx context.Context, id string) (bool, error) {
	result := r.db.WithContext(ctx).Model(&persistence.StealToken{}).Where("id = ? AND is_used = false", id).Update("is_used", true)
	return result.RowsAffected == 1, result.Error
}

func translateUserError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrUserNotFound
	}
	return err
}

func userFromRow(row persistence.User) identity.User {
	return identity.User{ID: row.ID, Email: row.Email, Name: row.Name, NickName: row.NickName,
		RoleID: row.RoleID, GroupID: row.GroupID, RemainingCoin: row.RemainingCoin,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
}

func usersFromRows(rows []persistence.User) []identity.User {
	users := make([]identity.User, len(rows))
	for i, row := range rows {
		users[i] = userFromRow(row)
	}
	return users
}

func splitCandidateIDs(csv string) []string {
	if csv == "" {
		return []string{}
	}
	parts := strings.Split(csv, ",")
	ids := make([]string, 0, len(parts))
	for _, id := range parts {
		if id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}
