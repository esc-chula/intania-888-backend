package user

import (
	"time"

	"github.com/esc-chula/intania-888-backend/internal/value"
)

// CreateInput describes the account fields supplied when creating a user.
type CreateInput struct {
	ID        string
	Email     string
	Name      string
	NickName  *string
	RoleID    string
	GroupID   *string
	CreatedAt time.Time
}

// ProfilePatch contains the editable fields of the authenticated user's profile.
// A nil Name leaves it unchanged. Nickname can be explicitly cleared.
type ProfilePatch struct {
	Name     *string
	NickName *string
	// NickNameSet distinguishes an omitted nickname from an explicit nil that clears it.
	NickNameSet bool
}

// AdminUpdateInput describes editable profile fields and the explicit balance.
// Role changes are performed by the operator database workflow.
type AdminUpdateInput struct {
	Name     string
	NickName *string
	GroupID  *string
	// GroupIDSet distinguishes an omitted group from an explicit nil that clears it.
	GroupIDSet    bool
	RemainingCoin value.Money
}
