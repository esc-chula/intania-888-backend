package user

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/esc-chula/intania-888-backend/internal/identity"
	persistence "github.com/esc-chula/intania-888-backend/internal/persistence/model"
)

type gormRepository struct{ db *gorm.DB }

// NewGORMRepository constructs an account repository backed by PostgreSQL/GORM.
func NewGORMRepository(db *gorm.DB) *gormRepository {
	return &gormRepository{db: db}
}

// Create persists an account snapshot and generated timestamps.
func (r *gormRepository) Create(ctx context.Context, user *identity.User) error {
	row := userRow(user)
	if err := r.db.WithContext(ctx).Create(row).Error; err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	*user = *userSnapshot(row)
	return nil
}

// GetByID retrieves an account snapshot by identifier.
func (r *gormRepository) GetByID(ctx context.Context, id string) (*identity.User, error) {
	var row persistence.User
	if err := r.db.WithContext(ctx).Preload("Role").Where("id = ?", id).First(&row).Error; err != nil {
		return nil, fmt.Errorf("get user by id: %w", mapUserLookupError(err))
	}
	return userSnapshot(&row), nil
}

// GetByEmail retrieves an account snapshot by email.
func (r *gormRepository) GetByEmail(ctx context.Context, email string) (*identity.User, error) {
	var row persistence.User
	if err := r.db.WithContext(ctx).Preload("Role").Where("email = ?", email).First(&row).Error; err != nil {
		return nil, fmt.Errorf("get user by email: %w", mapUserLookupError(err))
	}
	return userSnapshot(&row), nil
}

// GetAll retrieves all account snapshots.
func (r *gormRepository) GetAll(ctx context.Context) ([]*identity.User, error) {
	var rows []*persistence.User
	if err := r.db.WithContext(ctx).Preload("Role").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}

	users := make([]*identity.User, len(rows))
	for i, row := range rows {
		users[i] = userSnapshot(row)
	}
	return users, nil
}

// UpdateProfile writes only the permitted profile columns and leaves the balance untouched.
// Empty strings and nil pointers retain the existing field omission behavior.
func (r *gormRepository) UpdateProfile(ctx context.Context, input UpdateInput) error {
	updates := make(map[string]any, 5)

	if input.Email != "" {
		updates["email"] = input.Email
	}
	if input.Name != "" {
		updates["name"] = input.Name
	}
	if input.NickName != nil {
		updates["nick_name"] = *input.NickName
	}
	if input.RoleID != "" {
		updates["role_id"] = input.RoleID
	}
	if input.GroupID != nil {
		updates["group_id"] = *input.GroupID
	}

	if err := r.db.WithContext(ctx).Model(&persistence.User{}).
		Where("id = ?", input.ID).Updates(updates).Error; err != nil {
		return fmt.Errorf("update user profile: %w", err)
	}
	return nil
}

// Update applies the existing nonzero struct update semantics for administrator updates.
func (r *gormRepository) Update(ctx context.Context, user *identity.User) error {
	row := userRow(user)
	// Keep GORM's existing nonzero struct update semantics for normalization.
	if err := r.db.WithContext(ctx).Model(row).Where("id = ?", row.ID).Updates(row).Error; err != nil {
		return fmt.Errorf("update user: %w", err)
	}
	return nil
}

// WithinTransaction provides account operations on one database transaction.
func (r *gormRepository) WithinTransaction(ctx context.Context, fn func(Transaction) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&gormRepository{db: tx})
	})
}

// LockByID locks an account row until the transaction completes.
func (r *gormRepository) LockByID(ctx context.Context, id string) (*identity.User, error) {
	var row persistence.User
	if err := r.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ?", id).First(&row).Error; err != nil {
		return nil, fmt.Errorf("lock user: %w", mapUserLookupError(err))
	}
	return userSnapshot(&row), nil
}

// DeductBalance subtracts minor units from the locked account atomically.
func (r *gormRepository) DeductBalance(ctx context.Context, id string, amount int64) error {
	if err := r.db.WithContext(ctx).Model(&persistence.User{}).
		Where("id = ?", id).
		Update("remaining_coin", gorm.Expr("remaining_coin - ?", amount)).Error; err != nil {
		return fmt.Errorf("deduct coins: %w", err)
	}
	return nil
}

func mapUserLookupError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return errors.Join(ErrUserNotFound, err)
	}
	return err
}

func userRow(user *identity.User) *persistence.User {
	return &persistence.User{
		ID: user.ID, Email: user.Email, Name: user.Name,
		NickName: user.NickName, RoleID: user.RoleID, GroupID: user.GroupID,
		RemainingCoin: user.RemainingCoin, CreatedAt: user.CreatedAt, UpdatedAt: user.UpdatedAt,
	}
}

func userSnapshot(row *persistence.User) *identity.User {
	return &identity.User{
		ID: row.ID, Email: row.Email, Name: row.Name,
		NickName: row.NickName, RoleID: row.RoleID, GroupID: row.GroupID,
		RemainingCoin: row.RemainingCoin, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}
