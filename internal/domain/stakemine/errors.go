package stakemine

import "errors"

var (
	// ErrInvalidGameRequest indicates invalid game input.
	ErrInvalidGameRequest = errors.New("invalid Stake Mines request")
	// ErrInsufficientBalance indicates the account cannot fund the wager.
	ErrInsufficientBalance = errors.New("insufficient balance")
	// ErrGameNotFound indicates that a requested game does not exist.
	ErrGameNotFound = errors.New("stake mines game not found")
	// ErrNoActiveGame indicates the account has no active game.
	ErrNoActiveGame = errors.New("active Stake Mines game not found")
	// ErrGameConflict indicates a transition that the current state disallows.
	ErrGameConflict = errors.New("stake mines game state conflict")
	// ErrGameForbidden indicates a game belongs to another account.
	ErrGameForbidden = errors.New("stake mines game access forbidden")
	// ErrUserNotFound indicates the game's account does not exist.
	ErrUserNotFound = errors.New("user not found")
)
