package user

import (
	"encoding/json"
	"strings"
	"unicode/utf8"

	"github.com/esc-chula/intania-888-backend/internal/apierror"
	"github.com/esc-chula/intania-888-backend/internal/httpidentity"
	"github.com/esc-chula/intania-888-backend/internal/value"
)

const maxUserProfileFieldLength = 100

func addUserProfileFieldLengthError(details map[string]string, field, value string) {
	if utf8.RuneCountInString(value) > maxUserProfileFieldLength {
		details[field] = "must be at most 100 characters"
	}
}

// CreateUserRequest preserves the currently accepted user creation payload.
type CreateUserRequest httpidentity.ProfileResponse

// UpdateOwnProfileRequest contains only fields editable by the signed-in user.
// Omitted fields remain unchanged; null clears the nickname.
type UpdateOwnProfileRequest struct {
	Name     *string `json:"name"`
	NickName *string `json:"nick_name"`

	nameSet     bool
	nickNameSet bool
}

// RegisterProfileRequest contains the nickname and group chosen during registration.
type RegisterProfileRequest struct {
	NickName string `json:"nick_name"`
	GroupID  string `json:"group_id"`
}

// ValidateRequest requires nonblank registration fields that fit their database columns.
func (r RegisterProfileRequest) ValidateRequest() map[string]string {
	details := make(map[string]string)
	if strings.TrimSpace(r.NickName) == "" {
		details["nick_name"] = "is required"
	} else {
		addUserProfileFieldLengthError(details, "nick_name", r.NickName)
	}
	if strings.TrimSpace(r.GroupID) == "" {
		details["group_id"] = "is required"
	} else {
		addUserProfileFieldLengthError(details, "group_id", r.GroupID)
	}
	return details
}

// UnmarshalJSON records supplied fields and rejects identity and privilege fields.
func (r *UpdateOwnProfileRequest) UnmarshalJSON(data []byte) error {
	type wire UpdateOwnProfileRequest
	var decoded wire
	if err := apierror.DecodeKnownObject(data, &decoded, "name", "nick_name"); err != nil {
		return err
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	*r = UpdateOwnProfileRequest(decoded)
	_, r.nameSet = fields["name"]
	_, r.nickNameSet = fields["nick_name"]

	return nil
}

// AdminUpdateUserRequest intentionally excludes RoleID. Admin promotion and
// demotion are controlled by the operator database workflow.
type AdminUpdateUserRequest struct {
	Name          string      `json:"name" validate:"required"`
	NickName      *string     `json:"nick_name"`
	GroupID       *string     `json:"group_id"`
	RemainingCoin value.Money `json:"remaining_coin"`

	groupIDSet bool
}

// UnmarshalJSON records whether group_id was supplied so null can clear it.
func (r *AdminUpdateUserRequest) UnmarshalJSON(data []byte) error {
	type wire AdminUpdateUserRequest
	var decoded wire
	if err := apierror.DecodeKnownObject(data, &decoded, "name", "nick_name", "group_id", "remaining_coin"); err != nil {
		return err
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	*r = AdminUpdateUserRequest(decoded)
	_, r.groupIDSet = fields["group_id"]

	return nil
}

// DeductCoinRequest defines the HTTP payload for this account operation.
type DeductCoinRequest struct {
	Amount value.Money `json:"amount" validate:"required"`
}

// DeductCoinResponse defines the HTTP payload for this account operation.
type DeductCoinResponse struct {
	Success          bool        `json:"success"`
	DeductedAmount   value.Money `json:"deducted_amount"`
	RemainingBalance value.Money `json:"remaining_balance"`
}

// ValidateRequest rejects empty updates, invalid names, and overlong profile fields.
func (r UpdateOwnProfileRequest) ValidateRequest() map[string]string {
	details := make(map[string]string)
	if !r.nameSet && !r.nickNameSet {
		details["body"] = "must include at least one editable profile field"
	}
	if r.nameSet && (r.Name == nil || strings.TrimSpace(*r.Name) == "") {
		details["name"] = "must be a nonempty string"
	} else if r.nameSet {
		addUserProfileFieldLengthError(details, "name", *r.Name)
	}
	if r.nickNameSet && r.NickName != nil {
		addUserProfileFieldLengthError(details, "nick_name", *r.NickName)
	}
	return details
}

// ValidateRequest checks the required administrator-editable profile fields.
func (r AdminUpdateUserRequest) ValidateRequest() map[string]string {
	if strings.TrimSpace(r.Name) == "" {
		return map[string]string{"name": "is required"}
	}
	details := make(map[string]string)
	addUserProfileFieldLengthError(details, "name", r.Name)
	if r.NickName != nil {
		addUserProfileFieldLengthError(details, "nick_name", *r.NickName)
	}
	if r.groupIDSet && r.GroupID != nil && strings.TrimSpace(*r.GroupID) == "" {
		details["group_id"] = "must be a nonempty group ID or null"
	} else if r.groupIDSet && r.GroupID != nil {
		addUserProfileFieldLengthError(details, "group_id", *r.GroupID)
	}

	if len(details) == 0 {
		return nil
	}
	return details
}

// ValidateRequest checks the accepted HTTP deduction range.
func (r DeductCoinRequest) ValidateRequest() map[string]string {
	if r.Amount.MinorUnits() < 100 || r.Amount.MinorUnits() > 100_000_000 {
		return map[string]string{"amount": "must be between 1 and 1000000"}
	}

	return nil
}
