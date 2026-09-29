package location

import "context"

// ServicePort exposes catalogue reads and administrator management operations.
type ServicePort interface {
	GetAllLocations(context.Context) ([]*Location, error)
	GetLocation(context.Context, string) (*Location, error)
	CreateLocation(context.Context, Location) (*Location, error)
	UpdateLocation(context.Context, string, string) (*Location, error)
	DeleteLocation(context.Context, string) error
}

// Repository persists catalogue entries and translates storage errors.
type Repository interface {
	GetAllLocations(context.Context) ([]*Location, error)
	GetLocation(context.Context, string) (*Location, error)
	CreateLocation(context.Context, Location) (*Location, error)
	UpdateLocation(context.Context, string, string) (*Location, error)
	DeleteLocation(context.Context, string) error
}
