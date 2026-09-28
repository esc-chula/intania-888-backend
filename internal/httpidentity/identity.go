// Package httpidentity owns typed access to authenticated HTTP request state.
// Middleware installs identity snapshots in Fiber locals; feature HTTP adapters
// read them here and map profiles through the existing public JSON contract.
package httpidentity

import (
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/esc-chula/intania-888-backend/internal/identity"
	"github.com/esc-chula/intania-888-backend/internal/value"
)

const profileKey = "user"
const sessionIDKey = "session_id"
const csrfTokenKey = "csrf_token"

// ProfileResponse is the public profile shared by browser and external authentication.
// RemainingCoin is a fixed two-decimal JSON string; optional account values
// remain explicit nulls when absent. Session and CSRF credentials are excluded.
type ProfileResponse struct {
	ID            string      `json:"id"`
	Email         string      `json:"email"`
	Name          string      `json:"name"`
	NickName      *string     `json:"nick_name"`
	RoleID        string      `json:"role_id"`
	GroupID       *string     `json:"group_id"`
	RemainingCoin value.Money `json:"remaining_coin" swaggertype:"string" example:"888.88"`
	CreatedAt     time.Time   `json:"created_at"`
} // @name model.UserDto

// GetProfile returns the authenticated actor, or nil when no actor is installed.
func GetProfile(c *fiber.Ctx) *identity.Profile {
	profile, _ := c.Locals(profileKey).(*identity.Profile)
	return profile
}

// SetProfile installs an authenticated actor for downstream handlers.
func SetProfile(c *fiber.Ctx, profile *identity.Profile) { c.Locals(profileKey, profile) }

// SetSession installs the browser session ID and its CSRF token.
func SetSession(c *fiber.Ctx, id, csrfToken string) {
	c.Locals(sessionIDKey, id)
	c.Locals(csrfTokenKey, csrfToken)
}

// CSRFToken returns the token installed by browser authentication middleware.
// It returns an empty string when no browser session token has been installed.
func CSRFToken(c *fiber.Ctx) string {
	token, _ := c.Locals(csrfTokenKey).(string)
	return token
}

// Response maps an internal profile to its public HTTP representation.
// A nil profile produces a nil response.
func Response(profile *identity.Profile) *ProfileResponse {
	if profile == nil {
		return nil
	}
	return &ProfileResponse{
		ID: profile.ID, Email: profile.Email, Name: profile.Name,
		NickName: profile.NickName, RoleID: profile.RoleID, GroupID: profile.GroupID,
		RemainingCoin: profile.RemainingCoin, CreatedAt: profile.CreatedAt,
	}
}
