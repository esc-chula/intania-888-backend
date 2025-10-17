// package user

// import (
// 	"github.com/esc-chula/intania-888-backend/internal/model"
// 	"gorm.io/gorm"
// )

// type userRepositoryImpl struct {
// 	db *gorm.DB
// }

// func NewUserRepository(db *gorm.DB) UserRepository {
// 	return &userRepositoryImpl{db: db}
// }

// func (r *userRepositoryImpl) Create(user *model.User) error {
// 	return r.db.Create(user).Error
// }

// func (r *userRepositoryImpl) GetById(id string) (*model.User, error) {
// 	var user model.User
// 	if err := r.db.Preload("Role").Where("id = ?", id).First(&user).Error; err != nil {
// 		return nil, err
// 	}
// 	return &user, nil
// }

// func (r *userRepositoryImpl) GetByEmail(email string) (*model.User, error) {
// 	var user model.User
// 	if err := r.db.Preload("Role").Where("email = ?", email).First(&user).Error; err != nil {
// 		return nil, err
// 	}
// 	return &user, nil
// }

// func (r *userRepositoryImpl) GetAll() ([]*model.User, error) {
// 	var users []*model.User
// 	if err := r.db.Preload("Role").Find(&users).Error; err != nil {
// 		return nil, err
// 	}
// 	return users, nil
// }

// func (r *userRepositoryImpl) Update(user *model.User) error {
// 	return r.db.Model(user).Where("id = ?", user.Id).Updates(user).Error
// }

package user

import (
	"errors"

	"github.com/esc-chula/intania-888-backend/internal/model"
	"gorm.io/gorm"
)

type userRepositoryImpl struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepositoryImpl{db: db}
}

// Create inserts a new user
func (r *userRepositoryImpl) Create(user *model.User) error {
	sql := `
		INSERT INTO users (
			id, email, name, nick_name, role_id, group_id, remaining_coin, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
	`
	return r.db.Exec(sql,
		user.Id, user.Email, user.Name, user.NickName,
		user.RoleId, user.GroupId, user.RemainingCoin,
	).Error
}

// GetById retrieves user + role + group
func (r *userRepositoryImpl) GetById(id string) (*model.User, error) {
	var user model.User

	sql := `
		SELECT 
			u.id, u.email, u.name, u.nick_name, u.role_id, u.group_id, 
			u.remaining_coin, u.created_at, u.updated_at,
			r.id AS "role.id", r.created_at AS "role.created_at", r.updated_at AS "role.updated_at",
			g.id AS "group.id", g.color_id AS "group.color_id", 
			g.created_at AS "group.created_at", g.updated_at AS "group.updated_at"
		FROM users u
		LEFT JOIN roles r ON r.id = u.role_id
		LEFT JOIN intania_groups g ON g.id = u.group_id
		WHERE u.id = $1
		LIMIT 1
	`

	if err := r.db.Raw(sql, id).Scan(&user).Error; err != nil {
		return nil, err
	}
	if user.Id == "" {
		return nil, errors.New("user not found")
	}
	return &user, nil
}

// GetByEmail retrieves user by email + role + group
func (r *userRepositoryImpl) GetByEmail(email string) (*model.User, error) {
	var user model.User

	sql := `
		SELECT 
			u.id, u.email, u.name, u.nick_name, u.role_id, u.group_id, 
			u.remaining_coin, u.created_at, u.updated_at,
			r.id AS "role.id", r.created_at AS "role.created_at", r.updated_at AS "role.updated_at",
			g.id AS "group.id", g.color_id AS "group.color_id", 
			g.created_at AS "group.created_at", g.updated_at AS "group.updated_at"
		FROM users u
		LEFT JOIN roles r ON r.id = u.role_id
		LEFT JOIN intania_groups g ON g.id = u.group_id
		WHERE u.email = $1
		LIMIT 1
	`

	if err := r.db.Raw(sql, email).Scan(&user).Error; err != nil {
		return nil, err
	}
	if user.Id == "" {
		return nil, errors.New("user not found")
	}
	return &user, nil
}

// GetAll retrieves all users with their roles and groups
func (r *userRepositoryImpl) GetAll() ([]*model.User, error) {
	var users []*model.User

	sql := `
		SELECT 
			u.id, u.email, u.name, u.nick_name, u.role_id, u.group_id, 
			u.remaining_coin, u.created_at, u.updated_at,
			r.id AS "role.id", r.created_at AS "role.created_at", r.updated_at AS "role.updated_at",
			g.id AS "group.id", g.color_id AS "group.color_id", 
			g.created_at AS "group.created_at", g.updated_at AS "group.updated_at"
		FROM users u
		LEFT JOIN roles r ON r.id = u.role_id
		LEFT JOIN intania_groups g ON g.id = u.group_id
		ORDER BY u.created_at DESC
	`

	if err := r.db.Raw(sql).Scan(&users).Error; err != nil {
		return nil, err
	}

	return users, nil
}

// Update modifies existing user info
func (r *userRepositoryImpl) Update(user *model.User) error {
	sql := `
		UPDATE users
		SET 
			email = $1,
			name = $2,
			nick_name = $3,
			role_id = $4,
			group_id = $5,
			remaining_coin = $6,
			updated_at = NOW()
		WHERE id = $7
	`
	result := r.db.Exec(sql,
		user.Email, user.Name, user.NickName,
		user.RoleId, user.GroupId, user.RemainingCoin,
		user.Id,
	)

	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("user not found")
	}
	return nil
}
