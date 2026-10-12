// Package testutil provides integration helpers for a disposable PostgreSQL
// database. Helpers apply or roll back repository migrations and install temporary
// failure triggers; their connections must point to isolated test infrastructure.
package testutil
