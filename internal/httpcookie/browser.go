// Package httpcookie defines the browser cookie policy shared by HTTP adapters.
package httpcookie

import "github.com/gofiber/fiber/v2"

// SameSite permits localhost frontends to use a hosted development API.
// Development cookies must also use Secure because browsers require it for None.
func SameSite(production bool) string {
	if production {
		return fiber.CookieSameSiteLaxMode
	}

	return fiber.CookieSameSiteNoneMode
}
