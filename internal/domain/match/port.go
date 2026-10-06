package match

import (
	"context"
	"time"

	"github.com/esc-chula/intania-888-backend/internal/domain/teamcoin"
	"github.com/esc-chula/intania-888-backend/internal/value"
)

// Repository is the persistence required by ordinary match operations.
type Repository interface {
	// Create stores a new match.
	Create(context.Context, *Snapshot) error
	// GetByID loads one match.
	GetByID(context.Context, string) (*Snapshot, error)
	// GetAll loads matches using the supplied time for schedule filtering.
	GetAll(context.Context, *Filter, time.Time) ([]*Snapshot, error)
	// CountBetsForTeam counts pending bill selections.
	CountBetsForTeam(context.Context, string, string) (int64, error)
	// UpdateScore stores only the scores.
	UpdateScore(context.Context, *Snapshot) error
	// UpdateMatch stores editable match details.
	UpdateMatch(context.Context, *Snapshot, time.Time) error
	// Delete removes a match using existing database constraints.
	Delete(context.Context, string) error
}

// TransactionManager runs a settlement on one transaction-bound repository.
type TransactionManager interface {
	// WithinTransaction commits a successful callback and rolls back failures. The team coin
	// repository handed to the callback shares the same transaction.
	WithinTransaction(context.Context, func(TransactionRepository, teamcoin.Repository) error) error
}

// TransactionRepository is valid only during its transaction callback.
// Billing operations acquire the lifecycle guard, then match, bill, and user
// rows in sorted order, preserving the existing settlement lock protocol.
type TransactionRepository interface {
	// AcquireLifecycleLock serializes billing row discovery.
	AcquireLifecycleLock(context.Context) error
	// FindPendingBillIDs discovers pending bills containing a match.
	FindPendingBillIDs(context.Context, string) ([]string, error)
	// FindReferencedMatchIDs finds the matches in the given bills.
	FindReferencedMatchIDs(context.Context, []string) ([]string, error)
	// LockMatches loads matches in ID order with write locks.
	LockMatches(context.Context, []string) ([]Snapshot, error)
	// UpdateResult stores the terminal match result.
	UpdateResult(context.Context, *Snapshot, time.Time) error
	// LockBills loads bill heads in ID order with write locks.
	LockBills(context.Context, []string) ([]*BillSnapshot, error)
	// FindBillLines loads bill selections and their match states.
	FindBillLines(context.Context, []string) ([]BillLineSnapshot, error)
	// LockUsers loads account balances in ID order with write locks.
	LockUsers(context.Context, []string) ([]UserBalance, error)
	// UpdateBalance stores the balance of a locked account.
	UpdateBalance(context.Context, string, value.Money) error
	// SettleBill stores one terminal bill transition.
	SettleBill(context.Context, BillSettlement) error
	// CreateTerminalEvent inserts the audit event in the same transaction.
	CreateTerminalEvent(context.Context, TerminalEvent) error
}

// TeamCoins records the team coin ledger for a result inside the settlement transaction.
type TeamCoins interface {
	// Settle records the vote result of the match that was just decided.
	Settle(ctx context.Context, repo teamcoin.Repository, matchID string) error
}

// HTTPService is the set of use cases consumed by the match HTTP adapter.
type HTTPService interface {
	// CreateMatch creates a match from application input.
	CreateMatch(context.Context, *Input) error
	// GetMatch loads a match and computes its current rates.
	GetMatch(context.Context, string) (*Result, error)
	// GetTime returns the current UTC server time.
	GetTime() (string, error)
	// GetAllMatches loads matching results.
	GetAllMatches(context.Context, *Filter) ([]*Result, error)
	// UpdateMatchScore updates the scores without settling bills.
	UpdateMatchScore(context.Context, string, *ScoreInput) error
	// SetResult records a result and settles eligible bills atomically.
	SetResult(context.Context, string, *ResultInput) error
	// UpdateMatch updates the match details.
	UpdateMatch(context.Context, string, *Input) error
	// DeleteMatch removes a match.
	DeleteMatch(context.Context, string) error
}
