package security

// Session is the authenticated browser-session state.
// The cache adapter owns serialization; services operate on this snapshot.
// CreatedAt and ExpiresAt are Unix seconds. Roles are read from the current
// account rather than copied into a session.
type Session struct {
	UserID    string
	CreatedAt int64
	ExpiresAt int64
	CSRFToken string
}
