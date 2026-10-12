// Package billinglock provides the PostgreSQL transaction guard shared by bill
// placement, refund, and match settlement adapters. It serializes discovery of
// related billing rows before their deterministic match, bill, and user locks.
package billinglock
