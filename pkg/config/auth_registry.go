package config

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"unicode"

	"gopkg.in/yaml.v3"
)

// Application modes distinguish direct browser sessions from backend delegation.
const (
	CookieApplication = "cookie_session"
	CodeApplication   = "authorization_code"
)

// Supported scopes identify capabilities available to registered applications.
const (
	// ScopeProfileRead allows reading the authenticated account profile.
	ScopeProfileRead = "profile.read"

	// ScopeCoinsSpend allows deducting coins from the authenticated account.
	ScopeCoinsSpend = "coins.spend"
)

const (
	authRegistryVersion       = 1
	minimumClientSecretLength = 32
)

// AuthRegistry is the operator-managed list of first-party applications.
type AuthRegistry struct {
	Version int `yaml:"version"`
	Google  struct {
		CallbackURI string `yaml:"callback_uri"`
	} `yaml:"google"`
	Lifetimes    AuthLifetimes     `yaml:"lifetimes"`
	Applications []AuthApplication `yaml:"applications"`
}

// AuthLifetimes defines protocol and delegation durations in seconds.
type AuthLifetimes struct {
	Login    int `yaml:"login_transaction_seconds"`
	Code     int `yaml:"authorization_code_seconds"`
	Access   int `yaml:"access_token_seconds"`
	Idle     int `yaml:"delegation_idle_seconds"`
	Absolute int `yaml:"delegation_absolute_seconds"`
}

// AuthApplication contains one operator-approved application registration.
type AuthApplication struct {
	ID                string   `yaml:"id"`
	Mode              string   `yaml:"mode"`
	FrontendOrigin    string   `yaml:"frontend_origin"`
	DefaultReturnPath string   `yaml:"default_return_path"`
	OnboardingPath    string   `yaml:"onboarding_path"`
	LoginErrorPath    string   `yaml:"login_error_path"`
	RedirectURIs      []string `yaml:"redirect_uris"`
	ClientSecretEnv   string   `yaml:"client_secret_env"`
	AllowedScopes     []string `yaml:"allowed_scopes"`
	Secret            string   `yaml:"-"`
}

// Application looks up an application by its public identifier.
func (r *AuthRegistry) Application(id string) (AuthApplication, bool) {
	if r != nil {
		for _, app := range r.Applications {
			if app.ID == id {
				return app, true
			}
		}
	}

	return AuthApplication{}, false
}

// ResolveReturnPath accepts browser paths, never caller-selected authorities.
func ResolveReturnPath(origin, path string) (string, error) {
	decoded, err := url.PathUnescape(path)
	if err != nil ||
		!strings.HasPrefix(decoded, "/") ||
		strings.HasPrefix(decoded, "//") ||
		strings.Contains(decoded, "\\") ||
		strings.Contains(decoded, "#") ||
		strings.IndexFunc(decoded, unicode.IsControl) >= 0 {
		return "", fmt.Errorf("invalid return path")
	}

	base, err := url.Parse(origin)
	if err != nil {
		return "", err
	}

	relative, err := url.Parse(path)
	if err != nil || relative.IsAbs() || relative.Host != "" || relative.Fragment != "" {
		return "", fmt.Errorf("invalid return path")
	}

	resolved := base.ResolveReference(relative)
	if resolved.Scheme != base.Scheme || resolved.Host != base.Host {
		return "", fmt.Errorf("invalid return origin")
	}

	return resolved.String(), nil
}

// LoadAuthRegistry strictly decodes and validates the startup registry.
func LoadAuthRegistry(path, environment, cors string, resolveSecret func(string) string) (result *AuthRegistry, err error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("AUTH_CONFIG_FILE is required")
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open auth registry: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(true)

	var registry AuthRegistry
	if err := decoder.Decode(&registry); err != nil {
		return nil, fmt.Errorf("decode auth registry: %w", err)
	}

	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("auth registry must contain one YAML document")
	}

	for i := range registry.Applications {
		app := &registry.Applications[i]
		if app.ClientSecretEnv != "" {
			app.Secret = resolveSecret(app.ClientSecretEnv)
		}
	}

	if err := registry.Validate(environment, cors); err != nil {
		return nil, err
	}

	return &registry, nil
}

// Validate checks destinations and resolves explicitly referenced application secrets.
func (r *AuthRegistry) Validate(environment, cors string) error {
	production := strings.EqualFold(strings.TrimSpace(environment), "production")
	if r.Version != authRegistryVersion || len(r.Applications) == 0 {
		return fmt.Errorf("auth registry requires version %d and applications", authRegistryVersion)
	}

	t := r.Lifetimes
	for _, lifetime := range []struct {
		name    string
		seconds int
	}{
		{"login_transaction_seconds", t.Login},
		{"authorization_code_seconds", t.Code},
		{"access_token_seconds", t.Access},
		{"delegation_idle_seconds", t.Idle},
		{"delegation_absolute_seconds", t.Absolute},
	} {
		if lifetime.seconds <= 0 {
			return fmt.Errorf("lifetimes.%s must be positive", lifetime.name)
		}
	}

	if t.Absolute < t.Idle {
		return fmt.Errorf("lifetimes.delegation_absolute_seconds must be at least lifetimes.delegation_idle_seconds")
	}
	if t.Access > t.Absolute {
		return fmt.Errorf("lifetimes.access_token_seconds must not exceed lifetimes.delegation_absolute_seconds")
	}

	if err := authURL(r.Google.CallbackURI, production, false); err != nil {
		return fmt.Errorf("google.callback_uri: %w", err)
	}

	origins, err := validateConfiguredOrigins(cors)
	if err != nil {
		return err
	}

	seen := make(map[string]bool)
	for i := range r.Applications {
		app := &r.Applications[i]
		if app.ID == "" || strings.IndexFunc(app.ID, func(c rune) bool {
			return (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '-'
		}) >= 0 || seen[app.ID] {
			return fmt.Errorf("invalid or duplicate application ID")
		}
		seen[app.ID] = true

		switch app.Mode {
		case CookieApplication:
			if len(app.RedirectURIs) != 0 || app.ClientSecretEnv != "" || len(app.AllowedScopes) != 0 {
				return fmt.Errorf("cookie application %s has delegated settings", app.ID)
			}
			if err := authURL(app.FrontendOrigin, production, true); err != nil {
				return fmt.Errorf("application %s origin: %w", app.ID, err)
			}
			if _, ok := origins[app.FrontendOrigin]; !ok {
				return fmt.Errorf("CORS must include cookie application %s origin", app.ID)
			}
			for _, path := range []string{app.DefaultReturnPath, app.OnboardingPath, app.LoginErrorPath} {
				if _, err := ResolveReturnPath(app.FrontendOrigin, path); err != nil {
					return fmt.Errorf("application %s: %w", app.ID, err)
				}
			}

		case CodeApplication:
			if app.FrontendOrigin != "" ||
				app.DefaultReturnPath != "" ||
				app.OnboardingPath != "" ||
				app.LoginErrorPath != "" {
				return fmt.Errorf("code application %s has cookie settings", app.ID)
			}
			if len(app.RedirectURIs) == 0 || len(app.AllowedScopes) == 0 || app.ClientSecretEnv == "" {
				return fmt.Errorf("application %s requires callbacks, scopes and secret reference", app.ID)
			}
			if len(app.Secret) < minimumClientSecretLength {
				return fmt.Errorf("application %s secret must contain at least %d characters", app.ID, minimumClientSecretLength)
			}

			callbacks := make(map[string]bool)
			for _, uri := range app.RedirectURIs {
				if err := authURL(uri, production, false); err != nil || callbacks[uri] {
					return fmt.Errorf("application %s has invalid or duplicate callback", app.ID)
				}
				callbacks[uri] = true
			}

			scopes := make(map[string]bool)
			for _, scope := range app.AllowedScopes {
				if (scope != ScopeProfileRead && scope != ScopeCoinsSpend) || scopes[scope] {
					return fmt.Errorf("application %s has invalid or duplicate scope", app.ID)
				}
				scopes[scope] = true
			}

		default:
			return fmt.Errorf("application %s has invalid mode", app.ID)
		}
	}

	return nil
}

func authURL(raw string, production, origin bool) error {
	parsed, err := url.Parse(raw)
	if err != nil ||
		parsed.Host == "" ||
		strings.Contains(parsed.Host, "*") ||
		parsed.User != nil ||
		parsed.RawQuery != "" ||
		parsed.Fragment != "" ||
		strings.Contains(raw, "\\") {
		return fmt.Errorf("invalid absolute URL")
	}

	if parsed.Scheme != "https" &&
		(production ||
			parsed.Scheme != "http" ||
			(parsed.Hostname() != "localhost" &&
				parsed.Hostname() != "127.0.0.1" &&
				parsed.Hostname() != "::1")) {
		return fmt.Errorf("HTTPS required except local development")
	}

	if origin && parsed.Path != "" {
		return fmt.Errorf("frontend origin cannot contain a path")
	}

	return nil
}
