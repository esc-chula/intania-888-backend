// Package database constructs the PostgreSQL GORM connection used by repository
// adapters. Schema changes remain the responsibility of versioned migrations;
// constructing a connection does not migrate or seed the database.
package database
