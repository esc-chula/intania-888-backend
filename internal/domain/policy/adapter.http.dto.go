package policy

import (
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// CreatePolicyRequest is the create-policy HTTP payload.
type CreatePolicyRequest struct {
	Kind          string     `json:"kind" validate:"required,oneof=allowlist blacklist"`
	PrincipalType string     `json:"principal_type" validate:"required,oneof=email google_subject"`
	Principal     string     `json:"principal" validate:"required"`
	Reason        string     `json:"reason" validate:"required"`
	ExpiresAt     *time.Time `json:"expires_at"`
}

// UpdatePolicyRequest is the patch payload and preserves explicit null expiry.
type UpdatePolicyRequest struct {
	Reason       *string    `json:"reason"`
	ExpiresAt    *time.Time `json:"expires_at"`
	Enabled      *bool      `json:"enabled"`
	expiresAtSet bool
}

// UnmarshalJSON distinguishes missing expiry from an explicit null.
func (r *UpdatePolicyRequest) UnmarshalJSON(data []byte) error {
	type wire struct {
		Reason    *string `json:"reason"`
		Enabled   *bool   `json:"enabled"`
		ExpiresAt string  `json:"expires_at"`
	}
	var value wire
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	for field := range fields {
		if field != "reason" && field != "enabled" && field != "expires_at" {
			return errors.New("unknown field")
		}
	}
	r.Reason = value.Reason
	r.Enabled = value.Enabled
	r.ExpiresAt = nil
	r.expiresAtSet = false
	if raw, ok := fields["expires_at"]; ok {
		r.expiresAtSet = true
		if string(raw) != "null" {
			var expiresAt time.Time
			if err := json.Unmarshal(raw, &expiresAt); err != nil {
				return err
			}
			r.ExpiresAt = &expiresAt
		}
	}

	return nil
}

// ValidateRequest checks required fields before service validation.
func (r CreatePolicyRequest) ValidateRequest() map[string]string {
	details := make(map[string]string)
	if strings.TrimSpace(r.Kind) == "" {
		details["kind"] = "is required"
	}
	if strings.TrimSpace(r.PrincipalType) == "" {
		details["principal_type"] = "is required"
	}
	if strings.TrimSpace(r.Principal) == "" {
		details["principal"] = "is required"
	}
	if strings.TrimSpace(r.Reason) == "" {
		details["reason"] = "is required"
	}

	return details
}

// ValidateRequest rejects an empty policy update.
func (r UpdatePolicyRequest) ValidateRequest() map[string]string {
	if r.Reason == nil && !r.expiresAtSet && r.Enabled == nil {
		return map[string]string{"body": "must include at least one policy field"}
	}

	return nil
}

// ListResponse is the paginated HTTP response.
type ListResponse struct {
	Items      []*Response `json:"items"`
	NextCursor *string     `json:"next_cursor"`
}

// Response is the established access-policy HTTP representation.
type Response struct {
	ID            string     `json:"id"`
	Kind          string     `json:"kind"`
	PrincipalType string     `json:"principal_type"`
	Principal     string     `json:"principal"`
	Reason        string     `json:"reason"`
	Enabled       bool       `json:"enabled"`
	ExpiresAt     *time.Time `json:"expires_at"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// BootstrapFile is the operator policy-import wire format.
type BootstrapFile struct {
	Entries []BootstrapEntry `json:"entries"`
}

// BootstrapEntry is one policy in an operator import file.
type BootstrapEntry CreatePolicyRequest

// Input returns a transport-neutral policy command.
func (e BootstrapEntry) Input() CreateInput {
	return CreateInput(e)
}
