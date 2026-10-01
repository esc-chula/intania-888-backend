package location

import "context"

// ServicePort exposes catalogue reads and administrator management operations.
type ServicePort interface {
	// GetAllLocations returns the venue catalogue.
	GetAllLocations(context.Context) ([]*Location, error)
	// GetLocation returns a venue by ID.
	GetLocation(context.Context, string) (*Location, error)
	// CreateLocation adds a venue after validating its ID and title.
	CreateLocation(context.Context, Location) (*Location, error)
	// UpdateLocation changes a venue title by ID.
	UpdateLocation(context.Context, string, string) (*Location, error)
	// DeleteLocation removes a venue by ID when it is unused.
	DeleteLocation(context.Context, string) error
}

// Repository persists catalogue entries and translates storage errors.
type Repository interface {
	// GetAllLocations loads the venue catalogue.
	GetAllLocations(context.Context) ([]*Location, error)
	// GetLocation loads a venue by ID.
	GetLocation(context.Context, string) (*Location, error)
	// CreateLocation inserts a venue.
	CreateLocation(context.Context, Location) (*Location, error)
	// UpdateLocation stores a venue's title by ID.
	UpdateLocation(context.Context, string, string) (*Location, error)
	// DeleteLocation removes an unused venue by ID.
	DeleteLocation(context.Context, string) error
}
