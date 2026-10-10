package config

import (
	"fmt"
	"net/url"
	"strings"
)

// ValidateSecurity checks the configuration required by the cookie session and
// OAuth flow before the HTTP server starts. Production configuration is strict:
// missing redirects, weak secrets, wildcard origins, and insecure cookies are
// startup errors rather than runtime surprises.
func ValidateSecurity(cfg Config) error {
	if cfg == nil {
		return fmt.Errorf("configuration is nil")
	}

	if err := cfg.GetRateLimits().Validate(); err != nil {
		return err
	}

	server := cfg.GetServer()
	cache := cfg.GetCache()
	jwt := cfg.GetJWT()
	oauth := cfg.GetOAuth()
	session := cfg.GetSession()

	if strings.TrimSpace(server.Name) == "" {
		return fmt.Errorf("SERVER_NAME is required")
	}
	if strings.TrimSpace(jwt.AccessTokenSecret) == "" {
		return fmt.Errorf("JWT_ACCESS_TOKEN_SECRET is required")
	}
	if strings.TrimSpace(cache.Host) == "" || cache.Port <= 0 || cache.Port > 65535 {
		return fmt.Errorf("CACHE_HOST and CACHE_PORT are required")
	}
	if strings.TrimSpace(oauth.ClientID) == "" || strings.TrimSpace(oauth.ClientSecret) == "" {
		return fmt.Errorf("OAuth client ID and secret are required")
	}
	if oauth.Registry == nil {
		return fmt.Errorf("AUTH_CONFIG_FILE must load a valid auth registry")
	}
	if session.IdleTTLSeconds <= 0 {
		return fmt.Errorf("SESSION_IDLE_TTL_SECONDS must be positive")
	}
	if session.AbsoluteTTLSeconds <= 0 {
		return fmt.Errorf("SESSION_ABSOLUTE_TTL_SECONDS must be positive")
	}
	if session.AbsoluteTTLSeconds < session.IdleTTLSeconds {
		return fmt.Errorf("SESSION_ABSOLUTE_TTL_SECONDS must be at least SESSION_IDLE_TTL_SECONDS")
	}

	if err := oauth.Registry.Validate(server.Env, cfg.GetCORS().AllowOrigins); err != nil {
		return fmt.Errorf("auth registry: %w", err)
	}

	env := strings.ToLower(strings.TrimSpace(server.Env))
	if env != "development" && env != "production" {
		return fmt.Errorf("SERVER_ENV must be development or production")
	}
	if env == "production" {
		if len(jwt.AccessTokenSecret) < 32 {
			return fmt.Errorf("JWT_ACCESS_TOKEN_SECRET must contain at least 32 characters in production")
		}
		if strings.TrimSpace(server.URL) == "" {
			return fmt.Errorf("SERVER_URL is required in production")
		}
		if _, err := validateAbsoluteURL(server.URL, "SERVER_URL", true, false); err != nil {
			return err
		}
	}

	return nil
}

func validateAbsoluteURL(raw, field string, requireHTTPS, allowQuery bool) (string, error) {
	raw = strings.TrimSpace(raw)
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil ||
		parsed.Fragment != "" || (!allowQuery && parsed.RawQuery != "") {
		return "", fmt.Errorf("%s must be an absolute URL without credentials, fragments, or unsupported query parameters", field)
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", fmt.Errorf("%s must use http or https", field)
	}
	if requireHTTPS && scheme != "https" {
		return "", fmt.Errorf("%s must use https in production", field)
	}

	return scheme + "://" + strings.ToLower(parsed.Host), nil
}

func validateConfiguredOrigins(rawOrigins string) (map[string]struct{}, error) {
	origins := make(map[string]struct{})
	for _, rawOrigin := range strings.Split(rawOrigins, ",") {
		rawOrigin = strings.TrimSpace(rawOrigin)
		if rawOrigin == "" {
			continue
		}
		if rawOrigin == "*" {
			return nil, fmt.Errorf("CORS_ALLOW_ORIGINS cannot use wildcard origins with credentials")
		}
		origin, err := validateExactOrigin(rawOrigin)
		if err != nil {
			return nil, fmt.Errorf("invalid CORS_ALLOW_ORIGINS entry %q: %w", rawOrigin, err)
		}
		origins[origin] = struct{}{}
	}
	if len(origins) == 0 {
		return nil, fmt.Errorf("CORS_ALLOW_ORIGINS must contain at least one exact origin")
	}

	return origins, nil
}

func validateExactOrigin(raw string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil ||
		parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("origin must contain only scheme, host, and port")
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", fmt.Errorf("origin scheme must be http or https")
	}

	return scheme + "://" + strings.ToLower(parsed.Host), nil
}
