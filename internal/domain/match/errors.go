package match

import "errors"

var (
	// ErrResultConflict indicates a different result was already recorded.
	ErrResultConflict = errors.New("conflicting terminal result")
	// ErrInvalidResult indicates an invalid terminal result.
	ErrInvalidResult = errors.New("invalid match result")
	// ErrInvalidMatch indicates invalid match details.
	ErrInvalidMatch = errors.New("invalid match")
	// ErrInvalidLocation indicates a match references an unknown location.
	ErrInvalidLocation = errors.New("invalid match location")
	// ErrInvalidScore indicates invalid match scores.
	ErrInvalidScore = errors.New("invalid match score")
	// ErrNotFound indicates a missing match.
	ErrNotFound = errors.New("match not found")
)
