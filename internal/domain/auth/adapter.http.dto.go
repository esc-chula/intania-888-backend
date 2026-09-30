package auth

import (
	"github.com/esc-chula/intania-888-backend/internal/httpidentity"
)

// ExternalMeResponse returns the profile authenticated by an external Bearer token.
type ExternalMeResponse struct {
	Profile *httpidentity.ProfileResponse `json:"profile"`
}

// MeResponse is the browser profile response, including its session-bound CSRF token.
type MeResponse struct {
	Profile   *httpidentity.ProfileResponse `json:"profile"`
	CSRFToken string                        `json:"csrf_token"`
}
