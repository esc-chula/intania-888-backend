package user

import (
	"strings"

	"github.com/esc-chula/intania-888-backend/internal/httpidentity"
	"github.com/esc-chula/intania-888-backend/internal/value"
)

// CreateUserRequest preserves the currently accepted user creation payload.
type CreateUserRequest httpidentity.ProfileResponse

// UpdateUserRequest defines the HTTP payload for this account operation.
type UpdateUserRequest struct {
	ID       string  `json:"id"`
	Email    string  `json:"email"`
	Name     string  `json:"name" validate:"required"`
	NickName *string `json:"nick_name"`
	RoleID   string  `json:"role_id"`
	GroupID  *string `json:"group_id"`
} // @name model.UpdateUserDto

// AdminUpdateUserRequest intentionally excludes RoleID. Admin promotion and
// demotion are controlled by the operator database workflow.
type AdminUpdateUserRequest struct {
	Name          string      `json:"name" validate:"required"`
	NickName      *string     `json:"nick_name"`
	GroupID       *string     `json:"group_id"`
	RemainingCoin value.Money `json:"remaining_coin" swaggertype:"string" example:"888.88"`
} // @name model.AdminUpdateUserDto

// DeductCoinRequest defines the HTTP payload for this account operation.
type DeductCoinRequest struct {
	Amount value.Money `json:"amount" swaggertype:"string" validate:"required"`
} // @name model.DeductCoinRequest

// DeductCoinResponse defines the HTTP payload for this account operation.
type DeductCoinResponse struct {
	Success          bool        `json:"success"`
	DeductedAmount   value.Money `json:"deducted_amount" swaggertype:"string"`
	RemainingBalance value.Money `json:"remaining_balance" swaggertype:"string"`
} // @name model.DeductCoinResponse

// ValidateRequest checks the required profile fields.
func (r UpdateUserRequest) ValidateRequest() map[string]string {
	if strings.TrimSpace(r.Name) == "" {
		return map[string]string{"name": "is required"}
	}
	return nil
}

// ValidateRequest checks the required administrator-editable profile fields.
func (r AdminUpdateUserRequest) ValidateRequest() map[string]string {
	if strings.TrimSpace(r.Name) == "" {
		return map[string]string{"name": "is required"}
	}
	return nil
}

// ValidateRequest checks the accepted HTTP deduction range.
func (r DeductCoinRequest) ValidateRequest() map[string]string {
	if r.Amount.MinorUnits() < 100 || r.Amount.MinorUnits() > 100_000_000 {
		return map[string]string{"amount": "must be between 1 and 1000000"}
	}
	return nil
}
