package sporttype

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"

	persistence "github.com/esc-chula/intania-888-backend/internal/persistence/model"
)

// GORMRepository persists neutral sport catalogue entries in PostgreSQL.
type GORMRepository struct {
	db *gorm.DB
}

// NewGORMRepository constructs the sport catalogue query adapter.
func NewGORMRepository(db *gorm.DB) *GORMRepository {
	return &GORMRepository{
		db: db,
	}
}

// GetAllSportTypes loads every sport entry and maps persistence rows to catalogue values.
func (r *GORMRepository) GetAllSportTypes(ctx context.Context) ([]*SportType, error) {
	var sportTypes []*persistence.SportType

	if err := r.db.WithContext(ctx).Find(&sportTypes).Error; err != nil {
		return nil, err
	}

	rows := make([]*SportType, len(sportTypes))
	for i, row := range sportTypes {
		rows[i] = &SportType{
			ID:    row.ID,
			Title: row.Title,
		}
	}

	return rows, nil
}

// GetSportType loads an entry and translates a missing row.
func (r *GORMRepository) GetSportType(ctx context.Context, id string) (*SportType, error) {
	var row persistence.SportType
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&row).Error; err != nil {
		return nil, translateStorageError(err)
	}

	return &SportType{
		ID:    row.ID,
		Title: row.Title,
	}, nil
}

// CreateSportType inserts an entry, preserving duplicate-title support.
func (r *GORMRepository) CreateSportType(ctx context.Context, input SportType) (*SportType, error) {
	row := persistence.SportType{
		ID:    input.ID,
		Title: input.Title,
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return nil, translateStorageError(err)
	}

	return &SportType{
		ID:    row.ID,
		Title: row.Title,
	}, nil
}

// UpdateSportType renames and returns the entry in one statement.
func (r *GORMRepository) UpdateSportType(ctx context.Context, id, title string) (*SportType, error) {
	var row persistence.SportType
	result := r.db.WithContext(ctx).Raw(`
UPDATE sport_types SET title = ?, updated_at = now()
WHERE id = ? RETURNING id, title`, title, id).Scan(&row)
	if result.Error != nil {
		return nil, translateStorageError(result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, ErrSportTypeNotFound
	}

	return &SportType{
		ID:    row.ID,
		Title: row.Title,
	}, nil
}

// DeleteSportType relies on restrictive foreign keys, including during concurrent writes.
func (r *GORMRepository) DeleteSportType(ctx context.Context, id string) error {
	result := r.db.WithContext(ctx).Where("id = ?", id).Delete(&persistence.SportType{})
	if result.Error != nil {
		return translateStorageError(result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrSportTypeNotFound
	}

	return nil
}

func translateStorageError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrSportTypeNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return ErrSportTypeConflict
		case "23001", "23503":
			return ErrSportTypeInUse
		}
	}

	return err
}
