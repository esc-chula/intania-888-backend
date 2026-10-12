package httplimit

import (
	"fmt"
	"net/netip"
	"os"
	"strings"

	"github.com/gofiber/fiber/v2"
)

const clientIPKey = "rate_limit_client_ip"

// ValidateClientIPMode restricts forwarding-header trust to the Cloud Run boundary.
func ValidateClientIPMode(mode string) error {
	switch mode {
	case "", "direct":
		return nil
	case "cloud_run":
		if os.Getenv("K_SERVICE") == "" {
			return fmt.Errorf("SERVER_CLIENT_IP_MODE=cloud_run requires the Cloud Run K_SERVICE environment")
		}
		return nil
	default:
		return fmt.Errorf("SERVER_CLIENT_IP_MODE must be direct or cloud_run")
	}
}

// ResolveClientIP installs a canonical rate-limit key. Cloud Run mode assumes one
// platform-appended client address at the right edge; verify this in staging.
// Direct mode ignores every forwarding header, including X-Real-IP.
func ResolveClientIP(mode string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		peer := c.Context().RemoteIP().String()
		c.Locals(clientIPKey, resolveIP(mode, c.Get(fiber.HeaderXForwardedFor), peer))
		return c.Next()
	}
}

// ClientIP returns the resolved key, falling back to the canonical TCP peer.
func ClientIP(c *fiber.Ctx) string {
	if ip, ok := c.Locals(clientIPKey).(string); ok && ip != "" {
		return ip
	}
	return canonicalIP(c.Context().RemoteIP().String())
}

func resolveIP(mode, forwarded, peer string) string {
	if mode == "cloud_run" {
		values := strings.Split(forwarded, ",")
		if ip, err := netip.ParseAddr(strings.TrimSpace(values[len(values)-1])); err == nil && ip.Zone() == "" {
			return ip.Unmap().String()
		}
	}
	return canonicalIP(peer)
}

func canonicalIP(raw string) string {
	if ip, err := netip.ParseAddr(raw); err == nil {
		return ip.Unmap().String()
	}
	return raw
}
