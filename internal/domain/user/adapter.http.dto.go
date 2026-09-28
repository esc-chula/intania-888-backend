package user

import (
	"encoding/json"
	"strings"

	"github.com/esc-chula/intania-888-backend/internal/apierror"
	"github.com/esc-chula/intania-888-backend/internal/httpidentity"
	"github.com/esc-chula/intania-888-backend/internal/value"
)

// CreateUserRequest preserves the currently accepted user creation payload.
type CreateUserRequest httpidentity.ProfileResponse

// UpdateOwnProfileRequest contains only fields editable by the signed-in user.
// Omitted fields remain unchanged; null clears nickname or group.
type UpdateOwnProfileRequest struct {
	Name     *string `json:"name"`
	NickName *string `json:"nick_name" extensions:"x-nullable"`
	GroupID  *string `json:"group_id" extensions:"x-nullable"`

	nameSet     bool
	nickNameSet bool
	groupIDSet  bool
}

// UnmarshalJSON records supplied fields and rejects identity and privilege fields.
func (r *UpdateOwnProfileRequest) UnmarshalJSON(data []byte) error {
	type wire UpdateOwnProfileRequest
	var decoded wire
	if err := apierror.DecodeKnownObject(data, &decoded, "name", "nick_name", "group_id"); err != nil {
		return err
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	*r = UpdateOwnProfileRequest(decoded)
	_, r.nameSet = fields["name"]
	_, r.nickNameSet = fields["nick_name"]
	_, r.groupIDSet = fields["group_id"]
	return nil
}

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

// ValidateRequest rejects empty updates, null or blank names, and blank group IDs.
func (r UpdateOwnProfileRequest) ValidateRequest() map[string]string {
	details := make(map[string]string)
	if !r.nameSet && !r.nickNameSet && !r.groupIDSet {
		details["body"] = "must include at least one editable profile field"
	}
	if r.nameSet && (r.Name == nil || strings.TrimSpace(*r.Name) == "") {
		details["name"] = "must be a nonempty string"
	}
	if r.groupIDSet && r.GroupID != nil && strings.TrimSpace(*r.GroupID) == "" {
		details["group_id"] = "must be a nonempty group ID or null"
	}
	return details
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
