package bill

import "errors"

var (
	// ErrInvalidBill indicates invalid stake or selections.
	ErrInvalidBill = errors.New("invalid bill")
	// ErrMatchNotFound indicates a missing selected match.
	ErrMatchNotFound = errors.New("match not found")
	// ErrInsufficientBalance indicates a stake exceeds available funds.
	ErrInsufficientBalance = errors.New("insufficient balance")
	// ErrBillConflict indicates a disallowed terminal transition.
	ErrBillConflict = errors.New("bill lifecycle conflict")
	// ErrNotFound indicates a missing bill or its required related account.
	ErrNotFound = errors.New("bill not found")
)
