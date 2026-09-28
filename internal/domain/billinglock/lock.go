package billinglock

import "gorm.io/gorm"

// lifecycleLockKey serializes the short period in which billing operations
// discover related rows. The row locks that follow are still acquired in
// match, bill, and user order; this guard prevents two transactions from
// discovering different parts of the same accumulator concurrently.
const lifecycleLockKey int64 = 0x494e54414e494138

// Acquire serializes billing lifecycle discovery using a transaction advisory lock.
// The caller must supply an active transaction; PostgreSQL holds the lock until
// commit or rollback. Other dialects return nil without acquiring a lock.
func Acquire(tx *gorm.DB) error {
	if tx.Name() != "postgres" {
		return nil
	}

	return tx.Exec("SELECT pg_advisory_xact_lock(?)", lifecycleLockKey).Error
}
