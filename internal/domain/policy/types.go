package policy

import "time"

// Access policy kinds, principal categories, and list states.
const (
	// KindAllowlist permits a non-student email identity.
	KindAllowlist = "allowlist"
	// KindBlacklist blocks a matching identity before other access rules.
	KindBlacklist = "blacklist"

	// PrincipalEmail identifies a policy by normalized email.
	PrincipalEmail = "email"
	// PrincipalGoogleSubject identifies a policy by Google subject.
	PrincipalGoogleSubject = "google_subject"

	// StatusActive selects enabled and unexpired entries.
	StatusActive = "active"
	// StatusInactive selects disabled or expired entries.
	StatusInactive = "inactive"
	// StatusAll selects entries regardless of enabled state or expiry.
	StatusAll = "all"
)

// AccessPolicy is a transport-neutral access-control entry.
type AccessPolicy struct {
	ID            string
	Kind          string
	PrincipalType string
	Principal     string
	Reason        string
	Enabled       bool
	ExpiresAt     *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// CreateInput defines the identity and expiry of a new access policy.
type CreateInput struct {
	Kind          string
	PrincipalType string
	Principal     string
	Reason        string
	ExpiresAt     *time.Time
}

// UpdateInput distinguishes absent fields from explicit policy updates.
type UpdateInput struct {
	Reason       string
	ReasonSet    bool
	ExpiresAt    *time.Time
	ExpiresAtSet bool
	Enabled      bool
	EnabledSet   bool
}

// ListFilter selects and paginates access-policy entries.
type ListFilter struct {
	Kind          string
	PrincipalType string
	Status        string
	Limit         int
	Offset        int
}

// ListResult contains a policy page and whether another page exists.
type ListResult struct {
	Items   []*AccessPolicy
	HasMore bool
}
