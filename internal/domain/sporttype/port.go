package sporttype

import "context"

// ServicePort exposes the catalogue use case required by HTTP.
type ServicePort interface {
	// GetAllSportTypes returns the complete sport catalogue without transport or ORM metadata.
	GetAllSportTypes(context.Context) ([]*SportType, error)
}

// Repository loads the sport catalogue.
type Repository interface {
	// GetAllSportTypes returns the complete sport catalogue without transport or ORM metadata.
	GetAllSportTypes(context.Context) ([]*SportType, error)
}
