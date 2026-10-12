// Package cache provides Redis JSON storage and atomic browser session primitives.
// Feature adapters own cache keys and wire records; this package executes generic
// commands while preserving caller cancellation and operation timeouts.
package cache
