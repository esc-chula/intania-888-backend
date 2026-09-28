package auth

// OAuthLogin contains the authorization URL and browser-bound state nonce.
type OAuthLogin struct{ URL, State string }

// SessionCredentials identifies the session created by a successful login.
type SessionCredentials struct {
	SessionID string
	IsNewUser bool
}

// OAuthState contains the secret PKCE verifier retained during one login flow.
type OAuthState struct{ CodeVerifier string }
