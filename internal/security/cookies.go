package security

// Browser cookie names retain the local names and production __Host- prefixes.
const (
	LocalSessionCookieName  = "session"
	SecureSessionCookieName = "__Host-session"
	LocalOAuthCookieName    = "oauth"
	SecureOAuthCookieName   = "__Host-oauth"
)

// SessionCookieName selects the existing browser session cookie for the environment.
func SessionCookieName(production bool) string {
	if production {
		return SecureSessionCookieName
	}

	return LocalSessionCookieName
}

// OAuthCookieName selects the browser-bound OAuth state cookie for the environment.
func OAuthCookieName(production bool) string {
	if production {
		return SecureOAuthCookieName
	}

	return LocalOAuthCookieName
}
