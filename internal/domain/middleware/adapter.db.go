package middleware

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/esc-chula/intania-888-backend/internal/identity"
	persistence "github.com/esc-chula/intania-888-backend/internal/persistence/model"
)

// GORMRepository loads neutral account snapshots for request authentication.
type GORMRepository struct {
	db *gorm.DB
}

// NewGORMRepository constructs the account lookup adapter used by request authentication.
func NewGORMRepository(db *gorm.DB) *GORMRepository {
	return &GORMRepository{db}
}

// GetByID loads the account and role relation, preserving identity.ErrUserNotFound for an absent row.
func (r *GORMRepository) GetByID(ctx context.Context, id string) (*identity.User, error) {
	var user persistence.User
	if err := r.db.WithContext(ctx).Preload("Role").Where("id = ?", id).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("%w: %w", identity.ErrUserNotFound, err)
		}
		return nil, err
	}
	return &identity.User{
		ID:            user.ID,
		Email:         user.Email,
		Name:          user.Name,
		NickName:      user.NickName,
		RoleID:        user.RoleID,
		GroupID:       user.GroupID,
		RemainingCoin: user.RemainingCoin,
		CreatedAt:     user.CreatedAt,
		UpdatedAt:     user.UpdatedAt,
	}, nil
}
