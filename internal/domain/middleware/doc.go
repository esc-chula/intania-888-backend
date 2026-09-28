// Package middleware verifies request identities and enforces browser session,
// CSRF, blacklist, and administrator policy. External bearer authentication is a
// separate adapter and attaches the same neutral account profile to the request.
package middleware
