package location

import "errors"

var (
	// ErrInvalidLocation indicates an invalid venue ID or title.
	ErrInvalidLocation = errors.New("invalid location")
	// ErrLocationNotFound indicates that the requested venue does not exist.
	ErrLocationNotFound = errors.New("location not found")
	// ErrLocationConflict indicates that a venue ID already exists.
	ErrLocationConflict = errors.New("location already exists")
	// ErrLocationInUse indicates that a venue is referenced by a match.
	ErrLocationInUse = errors.New("location is in use")
)
