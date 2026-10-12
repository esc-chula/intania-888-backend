package event

import "errors"

var (
	// ErrDailyRewardOverrideNotFound indicates a missing date-specific reward.
	ErrDailyRewardOverrideNotFound = errors.New("daily reward override not found")
	// ErrDailyRewardAlreadyClaimed indicates an existing claim for the calendar date.
	ErrDailyRewardAlreadyClaimed = errors.New("daily reward already claimed")
	// ErrInsufficientBalance indicates insufficient coins for a spend or eligible raid.
	ErrInsufficientBalance = errors.New("insufficient balance")
	// ErrStealTokenInvalid indicates a missing or expired token.
	ErrStealTokenInvalid = errors.New("invalid or expired steal token")
	// ErrStealTokenConflict indicates that a token has already been consumed.
	ErrStealTokenConflict = errors.New("steal token already used")
	// ErrStealTokenForbidden indicates that a token belongs to another account.
	ErrStealTokenForbidden = errors.New("steal token is not owned by user")
	// ErrInvalidStealRequest indicates an invalid victim selection.
	ErrInvalidStealRequest = errors.New("invalid steal request")
	// ErrUserNotFound indicates a missing actor or selected victim.
	ErrUserNotFound = errors.New("event user not found")
)
