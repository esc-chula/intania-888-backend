package auth

import (
	"strings"

	"github.com/esc-chula/intania-888-backend/internal/httpidentity"
)

// LoginResponse returns the browser-bound OAuth authorization URL.
type LoginResponse struct {
	URL string `json:"url"`
}

// ExternalMeResponse returns the profile authenticated by an external Bearer token.
type ExternalMeResponse struct {
	Profile *httpidentity.ProfileResponse `json:"profile"`
}

// MeResponse is the browser profile response, including its session-bound CSRF token.
type MeResponse struct {
	Profile   *httpidentity.ProfileResponse `json:"profile"`
	CSRFToken string                        `json:"csrf_token"`
}

// ExternalTokenResponse is an issued external token and revocation identifier; ExpiresIn is seconds.
type ExternalTokenResponse struct {
	Token     string `json:"token"`
	ID        string `json:"id"`
	ExpiresIn int    `json:"expires_in"`
}

type externalTokenRequest struct {
	UserID string `json:"user_id" validate:"required"`
}

// ValidateRequest requires an existing-user identifier before token issuance reaches the service.
func (r externalTokenRequest) ValidateRequest() map[string]string {
	if strings.TrimSpace(r.UserID) == "" {
		return map[string]string{"user_id": "is required"}
	}

	return nil
}
