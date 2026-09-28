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

// UpdateInput describes a profile update for the authenticated account.
// Email and role are supplied from the authenticated actor, never the request body.
type UpdateInput struct {
	ID       string
	Email    string
	Name     string
	NickName *string
	RoleID   string
	GroupID  *string
}

// AdminUpdateInput describes editable profile fields and the explicit balance.
// Role changes are performed by the operator database workflow.
type AdminUpdateInput struct {
	Name          string
	NickName      *string
	GroupID       *string
	RemainingCoin value.Money
}
