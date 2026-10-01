package match

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/esc-chula/intania-888-backend/internal/domain/billinglock"
	persistence "github.com/esc-chula/intania-888-backend/internal/persistence/model"
	"github.com/esc-chula/intania-888-backend/internal/value"
)

type gormRepository struct{ db *gorm.DB }

// NewGORMRepository constructs ordinary and transaction-scoped match persistence.
func NewGORMRepository(db *gorm.DB) *gormRepository {
	return &gormRepository{db: db}
}

func matchLookupError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("%w: %w", ErrNotFound, err)
	}

	return err
}

// WithinTransaction binds the callback to one transaction and propagates commit errors.
func (r *gormRepository) WithinTransaction(ctx context.Context, fn func(TransactionRepository) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&gormRepository{db: tx})
	})
}

// Create stores the new match record.
func (r *gormRepository) Create(ctx context.Context, item *Snapshot) error {
	row := snapshotToRow(item)

	return matchWriteError(r.db.WithContext(ctx).Create(&row).Error)
}

// GetByID loads a match and translates missing rows to the feature error.
func (r *gormRepository) GetByID(ctx context.Context, id string) (*Snapshot, error) {
	var row persistence.Match

	if err := r.db.WithContext(ctx).Preload("Location").First(&row, "id = ?", id).Error; err != nil {
		return nil, matchLookupError(err)
	}

	result := snapshotFromRow(row)

	return &result, nil
}

// GetAll applies the existing schedule filters and start-time ordering.
func (r *gormRepository) GetAll(ctx context.Context, filter *Filter, now time.Time) ([]*Snapshot, error) {
	var rows []*persistence.Match
	query := r.db.WithContext(ctx).Preload("Location")

	if filter != nil {
		if filter.TypeID != "" {
			query = query.Where("type_id = ?", filter.TypeID)
		}

		switch filter.Schedule {
		case Schedule:
			query = query.Where("end_time > ?", now)
		case ScheduleResult:
			query = query.Where("end_time <= ?", now)
		}
	}

	if err := query.Order("start_time").Find(&rows).Error; err != nil {
		return nil, err
	}

	results := make([]*Snapshot, len(rows))
	for i, row := range rows {
		snapshot := snapshotFromRow(*row)
		results[i] = &snapshot
	}

	return results, nil
}

// CountBetsForTeam counts the existing pending bill selections.
func (r *gormRepository) CountBetsForTeam(ctx context.Context, matchID, teamID string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Table("bill_lines").
		Joins("JOIN bill_heads ON bill_heads.id = bill_lines.bill_id").
		Where("bill_lines.match_id = ? AND bill_lines.betting_on = ? AND bill_heads.status = 'PENDING'", matchID, teamID).Count(&count).Error

	return count, err
}

// UpdateScore writes only the score columns.
func (r *gormRepository) UpdateScore(ctx context.Context, item *Snapshot) error {
	updates := map[string]any{
		"teama_score": item.TeamAScore,
		"teamb_score": item.TeamBScore,
	}

	return r.db.WithContext(ctx).Model(&persistence.Match{ID: item.ID}).Updates(updates).Error
}

// UpdateMatch writes editable columns with the supplied update timestamp.
func (r *gormRepository) UpdateMatch(ctx context.Context, item *Snapshot, now time.Time) error {
	updates := map[string]any{
		"teama_id":    item.TeamAID,
		"teamb_id":    item.TeamBID,
		"type_id":     item.TypeID,
		"location_id": item.LocationID,
		"start_time":  item.StartTime,
		"end_time":    item.EndTime,
		"updated_at":  now,
	}

	return matchWriteError(r.db.WithContext(ctx).Model(&persistence.Match{ID: item.ID}).Updates(updates).Error)
}

func matchWriteError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23503" && pgErr.ConstraintName == "matches_location_id_fkey" {
		return ErrInvalidLocation
	}

	return err
}

// Delete preserves the existing affected-row not-found contract.
func (r *gormRepository) Delete(ctx context.Context, id string) error {
	result := r.db.WithContext(ctx).Delete(&persistence.Match{}, "id = ?", id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return matchLookupError(gorm.ErrRecordNotFound)
	}

	return nil
}

// AcquireLifecycleLock acquires the existing billing transaction advisory lock.
func (r *gormRepository) AcquireLifecycleLock(ctx context.Context) error {
	return billinglock.Acquire(r.db.WithContext(ctx))
}

// FindPendingBillIDs discovers pending bills containing the supplied match.
func (r *gormRepository) FindPendingBillIDs(ctx context.Context, matchID string) ([]string, error) {
	var ids []string
	err := r.db.WithContext(ctx).Table("bill_heads").
		Select("DISTINCT bill_heads.id").
		Joins("JOIN bill_lines ON bill_lines.bill_id=bill_heads.id").
		Where("bill_lines.match_id=? AND bill_heads.status='PENDING'", matchID).
		Order("bill_heads.id").
		Scan(&ids).Error

	return ids, err
}

// FindReferencedMatchIDs discovers the ordered match IDs in the given bills.
func (r *gormRepository) FindReferencedMatchIDs(ctx context.Context, billIDs []string) ([]string, error) {
	var ids []string
	err := r.db.WithContext(ctx).Table("bill_lines").
		Select("DISTINCT match_id").
		Where("bill_id IN ?", billIDs).
		Order("match_id").
		Scan(&ids).Error

	return ids, err
}

// LockMatches acquires match write locks in sorted ID order.
func (r *gormRepository) LockMatches(ctx context.Context, ids []string) ([]Snapshot, error) {
	var rows []persistence.Match

	if err := r.db.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id IN ?", ids).
		Order("id").
		Find(&rows).Error; err != nil {
		return nil, err
	}

	result := make([]Snapshot, len(rows))
	for i := range rows {
		result[i] = snapshotFromRow(rows[i])
	}

	return result, nil
}

// UpdateResult writes only the terminal match result and timestamp.
func (r *gormRepository) UpdateResult(ctx context.Context, item *Snapshot, now time.Time) error {
	updates := map[string]any{
		"winner_id":  item.WinnerID,
		"is_draw":    item.IsDraw,
		"updated_at": now,
	}

	return r.db.WithContext(ctx).Model(&persistence.Match{ID: item.ID}).Updates(updates).Error
}

// LockBills acquires bill head locks in sorted ID order.
func (r *gormRepository) LockBills(ctx context.Context, ids []string) ([]*BillSnapshot, error) {
	var rows []*persistence.BillHead

	if err := r.db.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id IN ?", ids).
		Order("id").
		Find(&rows).Error; err != nil {
		return nil, err
	}

	results := make([]*BillSnapshot, len(rows))
	for i, row := range rows {
		results[i] = &BillSnapshot{
			ID:     row.ID,
			UserID: row.UserID,
			Total:  value.MustMoneyFromMinor(row.Total),
			Status: row.Status,
		}
	}

	return results, nil
}

// FindBillLines loads selections and match states in the existing order.
func (r *gormRepository) FindBillLines(ctx context.Context, ids []string) ([]BillLineSnapshot, error) {
	var rows []persistence.BillLine

	if err := r.db.WithContext(ctx).
		Preload("Match").
		Preload("Match.Location").
		Where("bill_id IN ?", ids).
		Order("bill_id, match_id").
		Find(&rows).Error; err != nil {
		return nil, err
	}

	results := make([]BillLineSnapshot, len(rows))
	for i, row := range rows {
		results[i] = BillLineSnapshot{
			BillID:    row.BillID,
			MatchID:   row.MatchID,
			Rate:      value.MustRateFromMicro(row.Rate),
			BettingOn: row.BettingOn,
			Match:     snapshotFromRow(row.Match),
		}
	}

	return results, nil
}

// LockUsers acquires account write locks in ID order.
func (r *gormRepository) LockUsers(ctx context.Context, ids []string) ([]UserBalance, error) {
	var rows []persistence.User

	if err := r.db.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id IN ?", ids).
		Order("id").
		Find(&rows).Error; err != nil {
		return nil, err
	}

	results := make([]UserBalance, len(rows))
	for i, row := range rows {
		results[i] = UserBalance{
			ID:      row.ID,
			Balance: value.MustMoneyFromMinor(row.RemainingCoin),
		}
	}

	return results, nil
}

// UpdateBalance writes the balance of an already locked account.
func (r *gormRepository) UpdateBalance(ctx context.Context, id string, balance value.Money) error {
	return r.db.WithContext(ctx).
		Model(&persistence.User{ID: id}).
		Update("remaining_coin", balance.MinorUnits()).Error
}

// SettleBill persists the terminal bill transition and payout.
func (r *gormRepository) SettleBill(ctx context.Context, item BillSettlement) error {
	updates := map[string]any{
		"status":     item.Status,
		"payout":     item.Payout.MinorUnits(),
		"settled_at": item.SettledAt,
		"updated_at": item.SettledAt,
	}

	return r.db.WithContext(ctx).Model(&persistence.BillHead{ID: item.BillID}).Updates(updates).Error
}

// CreateTerminalEvent writes the existing settlement audit event.
func (r *gormRepository) CreateTerminalEvent(ctx context.Context, event TerminalEvent) error {
	row := persistence.BillTerminalEvent{
		ID:        event.ID,
		BillID:    event.BillID,
		Kind:      "SETTLED",
		Amount:    event.Amount.MinorUnits(),
		CreatedAt: event.CreatedAt,
	}

	return r.db.WithContext(ctx).Create(&row).Error
}
