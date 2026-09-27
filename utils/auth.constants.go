package utils

const (
	LocalSessionCookieName  = "session"
	SecureSessionCookieName = "__Host-session"
	LocalOAuthCookieName    = "oauth"
	SecureOAuthCookieName   = "__Host-oauth"
)

func SessionCookieName(production bool) string {
	if production {
		return SecureSessionCookieName
	}

	return LocalSessionCookieName
}

func OAuthCookieName(production bool) string {
	if production {
		return SecureOAuthCookieName
	}

	return LocalOAuthCookieName
}
