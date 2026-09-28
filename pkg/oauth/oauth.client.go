package oauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.uber.org/zap"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"github.com/esc-chula/intania-888-backend/pkg/config"
)

// GoogleOAuthClient is the Google authorization boundary consumed by authentication.
type GoogleOAuthClient interface {
	// GetUserInfo exchanges an authorization code with its PKCE verifier and loads the identity.
	GetUserInfo(ctx context.Context, code, codeVerifier string) (*GoogleUserInfo, error)
	// OAuthConfig returns the shared client configuration; callers must not mutate it during requests.
	OAuthConfig() *oauth2.Config
}

// GoogleClient exchanges Google OAuth codes and retrieves the provider identity.
// The OAuth configuration is retained by reference.
type GoogleClient struct {
	oauthConfig *oauth2.Config
	log         *zap.Logger
}

// NewGoogleOAuthClient constructs the Google adapter from its configuration and logger.
// It performs no network request and retains the supplied configuration by reference.
func NewGoogleOAuthClient(oauthConfig *oauth2.Config, log *zap.Logger) *GoogleClient {
	return &GoogleClient{
		oauthConfig: oauthConfig,
		log:         log,
	}
}

var (
	// ErrInvalidCode indicates missing authorization inputs or a failed code exchange.
	ErrInvalidCode = errors.New("invalid code")
	// ErrHTTP indicates a userinfo request failure or unsuccessful HTTP status.
	ErrHTTP = errors.New("unable to get user info")
	// ErrIO indicates failure to read the Google userinfo response.
	ErrIO = errors.New("unable to read google response")
	// ErrInvalidFormat indicates that the userinfo response is not the expected JSON shape.
	ErrInvalidFormat = errors.New("google sent unexpected format")
)

// GoogleUserInfo is the Google userinfo wire record returned after code exchange.
// VerifiedEmail is provider evidence; authentication decides whether the account may log in.
type GoogleUserInfo struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	VerifiedEmail bool   `json:"verified_email"`
	Name          string `json:"name"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
	Picture       string `json:"picture"`
	Locale        string `json:"locale"`
}

// GetUserInfo exchanges code with its PKCE verifier and fetches the Google identity.
// Caller cancellation is preserved within a ten-second timeout covering both requests.
// Provider failures are translated into the exported OAuth errors.
func (c *GoogleClient) GetUserInfo(ctx context.Context, code, codeVerifier string) (*GoogleUserInfo, error) {
	if c.oauthConfig == nil || code == "" || codeVerifier == "" {
		return nil, ErrInvalidCode
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	token, err := c.oauthConfig.Exchange(ctx, code, oauth2.VerifierOption(codeVerifier))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidCode, err)
	}

	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		"https://www.googleapis.com/oauth2/v2/userinfo",
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrHTTP, err)
	}
	request.Header.Set("Authorization", "Bearer "+token.AccessToken)

	resp, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrHTTP, err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			c.log.Named("GetUserEmail").Warn("Close Google userinfo response failed", zap.Error(closeErr))
		}
	}()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("%w: status %d", ErrHTTP, resp.StatusCode)
	}

	response, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrIO, err)
	}

	// var parsedResponse dto.GoogleUserEmailResponse
	var parsedResponse GoogleUserInfo
	if err = json.Unmarshal(response, &parsedResponse); err != nil {

		return nil, fmt.Errorf("%w: %w", ErrInvalidFormat, err)
	}

	return &parsedResponse, nil
}

// OAuthConfig returns the shared client configuration, not a defensive copy.
// Callers must not mutate the configuration while requests are in flight.
func (c *GoogleClient) OAuthConfig() *oauth2.Config {
	return c.oauthConfig
}

// LoadOAuthConfig builds the Google OAuth settings and email/profile scopes.
// The configured redirect URL is the backend callback destination.
func LoadOAuthConfig(cfg config.Config) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     cfg.GetOAuth().ClientID,
		ClientSecret: cfg.GetOAuth().ClientSecret,
		RedirectURL:  cfg.GetOAuth().RedirectURL,
		Endpoint:     google.Endpoint,
		Scopes:       []string{"https://www.googleapis.com/auth/userinfo.email", "https://www.googleapis.com/auth/userinfo.profile"},
	}
}
