package oauth

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"

	"github.com/esc-chula/intania-888-backend/pkg/config"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type GoogleOAuthClient interface {
	GetUserInfo(code string) (*GoogleUserInfo, error)
	OAuthConfig() *oauth2.Config
}

type googleOAuthClientImpl struct {
	oauthConfig *oauth2.Config
	log         *zap.Logger
}

func NewGoogleOAuthClient(oauthConfig *oauth2.Config, log *zap.Logger) GoogleOAuthClient {
	return &googleOAuthClientImpl{
		oauthConfig,
		log,
	}
}

var (
	InvalidCode   = errors.New("invalid code")
	HttpError     = errors.New("unable to get user info")
	IOError       = errors.New("unable to read google response")
	InvalidFormat = errors.New("google sent unexpected format")
)

type GoogleUserInfo struct {
	Id            string `json:"id"`
	Email         string `json:"email"`
	VerifiedEmail bool   `json:"verified_email"`
	Name          string `json:"name"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
	Picture       string `json:"picture"`
	Locale        string `json:"locale"`
}

func (c *googleOAuthClientImpl) GetUserInfo(code string) (*GoogleUserInfo, error) {
	token, err := c.oauthConfig.Exchange(context.TODO(), code)
	if err != nil {
		c.log.Named("GetUserEmail").Error("Exchange: ", zap.Error(err))
		return nil, InvalidCode
	}

	resp, err := http.Get("https://www.googleapis.com/oauth2/v2/userinfo?access_token=" + url.QueryEscape(token.AccessToken))
	if err != nil {
		c.log.Named("GetUserEmail").Error("Get: ", zap.Error(err))
		return nil, HttpError
	}
	defer resp.Body.Close()

	response, err := io.ReadAll(resp.Body)
	if err != nil {
		c.log.Named("GetUserEmail").Error("ReadAll: ", zap.Error(err))
		return nil, IOError
	}

	// var parsedResponse dto.GoogleUserEmailResponse
	var parsedResponse GoogleUserInfo
	if err = json.Unmarshal(response, &parsedResponse); err != nil {
		c.log.Named("GetUserEmail").Error("Unmarshal: ", zap.Error(err))
		return nil, InvalidFormat
	}

	return &parsedResponse, nil
}

func (c *googleOAuthClientImpl) OAuthConfig() *oauth2.Config {
	return c.oauthConfig
}

func LoadOAuthConfig(cfg config.Config) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     cfg.GetOAuth().ClientId,
		ClientSecret: cfg.GetOAuth().ClientSecret,
		RedirectURL:  cfg.GetOAuth().RedirectUrl,
		Endpoint:     google.Endpoint,
		Scopes:       []string{"https://www.googleapis.com/auth/userinfo.email", "https://www.googleapis.com/auth/userinfo.profile"},
	}
}
