package location

import "errors"

var (
	ErrInvalidLocation  = errors.New("invalid location")
	ErrLocationNotFound = errors.New("location not found")
	ErrLocationConflict = errors.New("location already exists")
	ErrLocationInUse    = errors.New("location is in use")
)
