package location

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"

	persistence "github.com/esc-chula/intania-888-backend/internal/persistence/model"
)

// GORMRepository persists catalogue entries in PostgreSQL.
type GORMRepository struct {
	db *gorm.DB
}

// NewGORMRepository constructs the venue query adapter.
func NewGORMRepository(db *gorm.DB) *GORMRepository {
	return &GORMRepository{db: db}
}

// GetAllLocations loads all entries in stable ID order.
func (r *GORMRepository) GetAllLocations(ctx context.Context) ([]*Location, error) {
	var rows []persistence.Location
	if err := r.db.WithContext(ctx).Order("id").Find(&rows).Error; err != nil {
		return nil, err
	}

	locations := make([]*Location, len(rows))
	for i, row := range rows {
		locations[i] = &Location{ID: row.ID, Title: row.Title}
	}

	return locations, nil
}

// GetLocation loads an entry and translates a missing row.
func (r *GORMRepository) GetLocation(ctx context.Context, id string) (*Location, error) {
	var row persistence.Location
	if err := r.db.WithContext(ctx).First(&row, "id = ?", id).Error; err != nil {
		return nil, translateStorageError(err)
	}

	return &Location{ID: row.ID, Title: row.Title}, nil
}

// CreateLocation inserts an entry. Duplicate titles are permitted.
func (r *GORMRepository) CreateLocation(ctx context.Context, input Location) (*Location, error) {
	row := persistence.Location{ID: input.ID, Title: input.Title}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return nil, translateStorageError(err)
	}

	return &Location{ID: row.ID, Title: row.Title}, nil
}

// UpdateLocation renames and returns the entry in one statement.
func (r *GORMRepository) UpdateLocation(ctx context.Context, id, title string) (*Location, error) {
	var row persistence.Location

	result := r.db.WithContext(ctx).Raw(`
UPDATE locations SET title = ?, updated_at = now()
WHERE id = ? RETURNING id, title`, title, id).Scan(&row)
	if result.Error != nil {
		return nil, translateStorageError(result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, ErrLocationNotFound
	}

	return &Location{ID: row.ID, Title: row.Title}, nil
}

// DeleteLocation relies on the match foreign key to reject referenced entries.
func (r *GORMRepository) DeleteLocation(ctx context.Context, id string) error {
	result := r.db.WithContext(ctx).Where("id = ?", id).Delete(&persistence.Location{})
	if result.Error != nil {
		return translateStorageError(result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrLocationNotFound
	}

	return nil
}

func translateStorageError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrLocationNotFound
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return ErrLocationConflict
		case "23001", "23503":
			return ErrLocationInUse
		}
	}

	return err
}
