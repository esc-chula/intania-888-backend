package sporttype

import "errors"

var (
	// ErrInvalidSportType indicates an invalid identifier or title.
	ErrInvalidSportType = errors.New("invalid sport type")
	// ErrSportTypeNotFound indicates a missing catalogue entry.
	ErrSportTypeNotFound = errors.New("sport type not found")
	// ErrSportTypeConflict indicates an existing identifier.
	ErrSportTypeConflict = errors.New("sport type already exists")
	// ErrSportTypeInUse indicates a match, tournament group, or stage reference.
	ErrSportTypeInUse = errors.New("sport type is in use")
)
