package bill

import (
	"context"
	"time"

	"github.com/esc-chula/intania-888-backend/internal/domain/match"
	"github.com/esc-chula/intania-888-backend/internal/domain/teamcoin"
	"github.com/esc-chula/intania-888-backend/internal/value"
)

// Repository supplies persisted bill snapshots for read use cases.
type Repository interface {
	// GetByID loads a bill, restricting ownership when userID is nonempty.
	GetByID(context.Context, string, string) (*Result, error)
	// GetAll loads the user's bills in creation order.
	GetAll(context.Context, string) ([]*Result, error)
	// GetAllAdmin loads all bills for administrators.
	GetAllAdmin(context.Context) ([]*Result, error)
}

// TransactionManager runs bill transitions against one database transaction.
type TransactionManager interface {
	// WithinTransaction commits successful callbacks and rolls back failures. The team coin
	// repository handed to the callback shares the same transaction.
	WithinTransaction(context.Context, func(TransactionRepository, teamcoin.Repository) error) error
}

// TransactionRepository is valid only inside its transaction callback.
// Acquire the billing lifecycle guard before discovering rows, then lock match,
// bill, and user rows in ID order to preserve the billing lock protocol.
type TransactionRepository interface {
	// AcquireLifecycleLock serializes billing row discovery.
	AcquireLifecycleLock(context.Context) error
	// LockMatches loads selected match rows in ID order with write locks.
	LockMatches(context.Context, []string) ([]match.Snapshot, error)
	// CountBets groups pending selections for the requested match.
	CountBets(context.Context, string) ([]BetCount, error)
	// LockBalance reads an account balance under a write lock.
	LockBalance(context.Context, string) (value.Money, error)
	// LockVoteColor reads the color of the locked account's current group.
	LockVoteColor(context.Context, string) (*string, error)
	// CreateBill stores the bill and its lines, omitting nested match writes.
	CreateBill(context.Context, *Result) error
	// DebitBalance conditionally debits an account with sufficient balance.
	DebitBalance(context.Context, string, value.Money) error
	// FindMatchIDs discovers the ordered matches referenced by a bill.
	FindMatchIDs(context.Context, string) ([]string, error)
	// LockBill loads a bill and its lines under a write lock.
	LockBill(context.Context, string) (*Result, error)
	// VoidBill persists the terminal void transition.
	VoidBill(context.Context, string, value.Money, time.Time) error
	// UpdateBalance stores the balance of a locked account.
	UpdateBalance(context.Context, string, value.Money) error
	// CreateTerminalEvent inserts a void audit event in the same transaction.
	CreateTerminalEvent(context.Context, TerminalEvent) error
}

// TeamCoins corrects the team coin ledger when a bill is voided.
type TeamCoins interface {
	// Adjust re-evaluates a decided match's ledger after billID was voided.
	Adjust(ctx context.Context, repo teamcoin.Repository, matchID, billID string) error
}

// HTTPService is the bill use cases consumed by the HTTP adapter.
type HTTPService interface {
	// CreateBill places an authoritative bill and debits its stake atomically.
	CreateBill(context.Context, string, *CreateInput) (*Result, error)
	// GetBill loads a bill belonging to the user.
	GetBill(context.Context, string, string) (*Result, error)
	// GetAllBills loads the user's bills.
	GetAllBills(context.Context, string) ([]*Result, error)
	// GetAllBillsAdmin loads all bills for an administrator.
	GetAllBillsAdmin(context.Context) ([]*Result, error)
	// VoidBill refunds a pending bill and records the actor and reason.
	VoidBill(context.Context, string, string, string) (*Result, error)
}
