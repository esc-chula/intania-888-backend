// Package model defines shared GORM records and relationships for database adapters.
// Coin amounts use int64 hundredth units and stored rates use int64 millionth
// units. Pointer fields preserve SQL NULL, and association fields are populated
// only when loaded. Goose migrations own the physical schema and constraints;
// adapters translate these records into feature-owned application snapshots.
package model
