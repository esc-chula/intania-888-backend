package stakemine

import "context"

// ServicePort describes game use cases consumed by the HTTP adapter.
type ServicePort interface {
	// CreateGame creates a game and funds its wager atomically.
	CreateGame(context.Context, string, CreateInput) (*GameResult, error)
	// RevealTile reveals a tile and commits any terminal payout and history together.
	RevealTile(context.Context, string, string, RevealInput) (*GameResult, string, error)
	// CashOut ends an active game and credits its payout once.
	CashOut(context.Context, string, string) (*GameResult, error)
	// GetGame retrieves an owned game.
	GetGame(context.Context, string, string) (*GameResult, error)
	// GetActiveGame retrieves the account's active game.
	GetActiveGame(context.Context, string) (*GameResult, error)
	// GetGameHistory retrieves games with the existing pagination order.
	GetGameHistory(context.Context, string, int, int) ([]HistoryResult, error)
	// GetStats retrieves realized totals and current active exposure.
	GetStats(context.Context, string) (*StatsResult, error)
}

// Repository provides game snapshots and transaction boundaries.
// Missing lookups return feature errors and preserve underlying storage errors.
type Repository interface {
	// FindByID retrieves game metadata and decoded tiles; GridError must be checked after ownership.
	FindByID(context.Context, string) (*Game, error)
	// FindActiveByUserID retrieves the account's active game.
	FindActiveByUserID(context.Context, string) (*Game, error)
	// FindByUserID retrieves games newest first.
	FindByUserID(context.Context, string, int, int) ([]Game, error)
	// GetStatsByUserID retrieves realized and active game totals.
	GetStatsByUserID(context.Context, string) (*StatsResult, error)
	// WithinTransaction commits the callback's changes together or rolls them back.
	WithinTransaction(context.Context, func(Transaction) error) error
}

// Transaction provides operations bound to a single game transaction.
// Callbacks must not retain it after returning.
type Transaction interface {
	// LockUserBalance locks and returns an account balance in minor units.
	LockUserBalance(context.Context, string) (int64, error)
	// CountActiveGames counts games while the account lock is held.
	CountActiveGames(context.Context, string) (int64, error)
	// LockGame locks game metadata; GridError must be checked after ownership and state guards.
	LockGame(context.Context, string) (*Game, error)
	// CreateGame persists a new game and generated timestamps.
	CreateGame(context.Context, *Game) error
	// SaveGame persists a locked game state.
	SaveGame(context.Context, *Game) error
	// AdjustBalance adds signed minor units atomically to the account.
	AdjustBalance(context.Context, string, int64) error
	// CreateHistory records one tile reveal within the same transaction.
	CreateHistory(context.Context, *History) error
}
