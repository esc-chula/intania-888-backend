package bill

import (
	"context"
	"strings"
)

// VoidBill atomically refunds a pending bill and records its audit event.
// Repeating a void leaves the already voided bill unchanged.
func (s *Service) VoidBill(ctx context.Context, id, actor, reason string) (*Result, error) {
	actor = strings.TrimSpace(actor)
	reason = strings.TrimSpace(reason)
	if actor == "" || len(reason) == 0 || len(reason) > 500 {
		return nil, ErrInvalidBill
	}

	var bill *Result

	err := s.transactions.WithinTransaction(ctx, func(tx TransactionRepository) error {
		if err := tx.AcquireLifecycleLock(ctx); err != nil {
			return err
		}

		matchIDs, err := tx.FindMatchIDs(ctx, id)
		if err != nil {
			return err
		}
		if len(matchIDs) > 0 {
			matches, err := tx.LockMatches(ctx, matchIDs)
			if err != nil {
				return err
			}
			if len(matches) != len(matchIDs) {
				return ErrNotFound
			}
		}

		bill, err = tx.LockBill(ctx, id)
		if err != nil {
			return err
		}

		if bill.Status == StatusVoided {
			return nil
		}
		if bill.Status != StatusPending {
			return ErrBillConflict
		}

		balance, err := tx.LockBalance(ctx, bill.UserID)
		if err != nil {
			return err
		}
		balance, err = balance.Add(bill.Total)
		if err != nil {
			return err
		}

		now := s.now()
		if err := tx.VoidBill(ctx, bill.ID, bill.Total, now); err != nil {
			return err
		}
		if err := tx.UpdateBalance(ctx, bill.UserID, balance); err != nil {
			return err
		}
		event := TerminalEvent{
			ID:        s.newID(),
			BillID:    bill.ID,
			Amount:    bill.Total,
			ActorID:   actor,
			Reason:    reason,
			CreatedAt: now,
		}
		if err := tx.CreateTerminalEvent(ctx, event); err != nil {
			return err
		}
		bill.Status = StatusVoided
		payout := bill.Total
		bill.Payout = &payout
		bill.VoidedAt = &now

		return nil
	})
	if err != nil {
		return nil, err
	}

	return bill, nil
}
