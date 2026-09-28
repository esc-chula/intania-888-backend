package bill

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/esc-chula/intania-888-backend/internal/domain/billinglock"
	"github.com/esc-chula/intania-888-backend/internal/domain/match"
	persistence "github.com/esc-chula/intania-888-backend/internal/persistence/model"
	"github.com/esc-chula/intania-888-backend/internal/value"
)

type gormRepository struct{ db *gorm.DB }

// NewGORMRepository constructs bill reads and transaction-scoped persistence.
func NewGORMRepository(db *gorm.DB) *gormRepository {
	return &gormRepository{db: db}
}

// WithinTransaction binds all callback methods to a single GORM transaction.
func (r *gormRepository) WithinTransaction(ctx context.Context, fn func(TransactionRepository) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&gormRepository{db: tx})
	})
}

func billLookupError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("%w: %w", ErrNotFound, err)
	}
	return err
}

func preload(db *gorm.DB) *gorm.DB { return db.Preload("Lines").Preload("Lines.Match") }

// GetByID loads a bill, optionally restricting its owner.
func (r *gormRepository) GetByID(ctx context.Context, id, userID string) (*Result, error) {
	var row persistence.BillHead
	query := preload(r.db.WithContext(ctx)).Where("id = ?", id)
	if userID != "" {
		query = query.Where("user_id = ?", userID)
	}
	if err := query.First(&row).Error; err != nil {
		return nil, billLookupError(err)
	}
	return billFromRow(&row), nil
}

// GetAll loads the user's bills in descending creation order.
func (r *gormRepository) GetAll(ctx context.Context, userID string) ([]*Result, error) {
	var rows []*persistence.BillHead
	if err := preload(r.db.WithContext(ctx)).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return billsFromRows(rows), nil
}

// GetAllAdmin loads all bills and the existing user associations.
func (r *gormRepository) GetAllAdmin(ctx context.Context) ([]*Result, error) {
	var rows []*persistence.BillHead
	if err := preload(r.db.WithContext(ctx)).
		Preload("User").
		Order("created_at DESC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return billsFromRows(rows), nil
}

// AcquireLifecycleLock acquires the existing PostgreSQL transaction advisory lock.
func (r *gormRepository) AcquireLifecycleLock(ctx context.Context) error {
	return billinglock.Acquire(r.db.WithContext(ctx))
}

// LockMatches acquires the selected match row locks in ID order.
func (r *gormRepository) LockMatches(ctx context.Context, ids []string) ([]match.Snapshot, error) {
	var rows []persistence.Match
	if err := r.db.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id IN ?", ids).
		Order("id").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	snapshots := make([]match.Snapshot, len(rows))
	for i := range rows {
		snapshots[i] = matchSnapshot(rows[i])
	}
	return snapshots, nil
}

// CountBets groups pending bill selections for a match.
func (r *gormRepository) CountBets(ctx context.Context, matchID string) ([]BetCount, error) {
	var counts []BetCount
	err := r.db.WithContext(ctx).Table("bill_lines").
		Select("bill_lines.betting_on, count(*) AS count").
		Joins("JOIN bill_heads ON bill_heads.id = bill_lines.bill_id").
		Where("bill_lines.match_id = ? AND bill_heads.status = 'PENDING'", matchID).
		Group("bill_lines.betting_on").
		Scan(&counts).Error
	return counts, err
}

// LockBalance loads an account under a write lock.
func (r *gormRepository) LockBalance(ctx context.Context, userID string) (value.Money, error) {
	var row persistence.User
	if err := r.db.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		First(&row, "id = ?", userID).Error; err != nil {
		return value.Money{}, billLookupError(err)
	}
	return value.MustMoneyFromMinor(row.RemainingCoin), nil
}

// CreateBill writes a head and its lines without writing the nested matches.
func (r *gormRepository) CreateBill(ctx context.Context, bill *Result) error {
	row := billToRow(bill)
	return r.db.WithContext(ctx).Omit("Lines.Match").Create(&row).Error
}

// DebitBalance preserves the existing guarded SQL debit and affected-row check.
func (r *gormRepository) DebitBalance(ctx context.Context, userID string, amount value.Money) error {
	result := r.db.WithContext(ctx).
		Model(&persistence.User{}).
		Where("id = ? AND remaining_coin >= ?", userID, amount.MinorUnits()).
		Update("remaining_coin", gorm.Expr("remaining_coin - ?", amount.MinorUnits()))
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrInsufficientBalance
	}
	return nil
}

// FindMatchIDs discovers a bill's match rows in lock order.
func (r *gormRepository) FindMatchIDs(ctx context.Context, billID string) ([]string, error) {
	var ids []string
	err := r.db.WithContext(ctx).Table("bill_lines").
		Select("match_id").
		Where("bill_id = ?", billID).
		Order("match_id").
		Scan(&ids).Error
	return ids, err
}

// LockBill reads a bill under a write lock with its line and match associations.
func (r *gormRepository) LockBill(ctx context.Context, billID string) (*Result, error) {
	var row persistence.BillHead
	if err := preload(r.db.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"})).
		First(&row, "id = ?", billID).Error; err != nil {
		return nil, billLookupError(err)
	}
	return billFromRow(&row), nil
}

// VoidBill stores the void transition without changing any other bill fields.
func (r *gormRepository) VoidBill(ctx context.Context, billID string, payout value.Money, now time.Time) error {
	updates := map[string]any{
		"status":     StatusVoided,
		"payout":     payout.MinorUnits(),
		"voided_at":  now,
		"updated_at": now,
	}
	return r.db.WithContext(ctx).Model(&persistence.BillHead{ID: billID}).Updates(updates).Error
}

// UpdateBalance stores a balance whose account was locked by the callback.
func (r *gormRepository) UpdateBalance(ctx context.Context, userID string, balance value.Money) error {
	return r.db.WithContext(ctx).
		Model(&persistence.User{ID: userID}).
		Update("remaining_coin", balance.MinorUnits()).Error
}

// CreateTerminalEvent stores the void audit event in the transaction.
func (r *gormRepository) CreateTerminalEvent(ctx context.Context, event TerminalEvent) error {
	row := persistence.BillTerminalEvent{
		ID:        event.ID,
		BillID:    event.BillID,
		Kind:      StatusVoided,
		Amount:    event.Amount.MinorUnits(),
		ActorID:   &event.ActorID,
		Reason:    &event.Reason,
		CreatedAt: event.CreatedAt,
	}
	return r.db.WithContext(ctx).Create(&row).Error
}
