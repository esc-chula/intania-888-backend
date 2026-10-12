// Package sporttype provides the sport catalogue and stable codes shared with
// seed data and persisted matches. HTTP records and PostgreSQL rows are mapped
// to neutral catalogue values at their adapter boundaries.
// Administrators can create and rename entries, or delete unreferenced entries.
// Restrictive database foreign keys preserve matches, groups, and stage records.
package sporttype
