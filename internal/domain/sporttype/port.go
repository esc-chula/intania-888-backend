package sporttype

import "context"

// ServicePort exposes catalogue reads and administrator management use cases.
type ServicePort interface {
	// GetAllSportTypes returns the complete sport catalogue without transport or ORM metadata.
	GetAllSportTypes(context.Context) ([]*SportType, error)
	// GetSportType returns one sport or ErrSportTypeNotFound.
	GetSportType(context.Context, string) (*SportType, error)
	// CreateSportType validates and creates an entry with an immutable ID.
	CreateSportType(context.Context, SportType) (*SportType, error)
	// UpdateSportType renames an existing entry without changing its ID.
	UpdateSportType(context.Context, string, string) (*SportType, error)
	// DeleteSportType removes an unused entry or returns ErrSportTypeInUse.
	DeleteSportType(context.Context, string) error
}

// Repository persists the sport catalogue and translates storage failures.
type Repository interface {
	// GetAllSportTypes returns the complete sport catalogue without transport or ORM metadata.
	GetAllSportTypes(context.Context) ([]*SportType, error)
	// GetSportType returns one sport or ErrSportTypeNotFound.
	GetSportType(context.Context, string) (*SportType, error)
	// CreateSportType inserts an entry or returns ErrSportTypeConflict.
	CreateSportType(context.Context, SportType) (*SportType, error)
	// UpdateSportType atomically renames and returns the entry.
	UpdateSportType(context.Context, string, string) (*SportType, error)
	// DeleteSportType uses database constraints to reject referenced entries.
	DeleteSportType(context.Context, string) error
}
