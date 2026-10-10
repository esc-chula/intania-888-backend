package auth

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/esc-chula/intania-888-backend/internal/apierror"
	"github.com/esc-chula/intania-888-backend/internal/domain/middleware"
	"github.com/esc-chula/intania-888-backend/internal/httpcookie"
	"github.com/esc-chula/intania-888-backend/internal/httplimit"
	"github.com/esc-chula/intania-888-backend/internal/identity"
	"github.com/esc-chula/intania-888-backend/internal/security"
	"github.com/esc-chula/intania-888-backend/pkg/cache"
	"github.com/esc-chula/intania-888-backend/pkg/config"
)

// OAuth error values are returned by the application authorization protocol.
const (
	oauthErrorInvalidRequest         = "invalid_request"
	oauthErrorInvalidScope           = "invalid_scope"
	oauthErrorInvalidClient          = "invalid_client"
	oauthErrorInvalidGrant           = "invalid_grant"
	oauthErrorUnsupportedGrantType   = "unsupported_grant_type"
	oauthErrorAccessDenied           = "access_denied"
	oauthErrorTemporarilyUnavailable = "temporarily_unavailable"
)

// Fixed protocol values and safety limits are shared across the login handlers.
const (
	maxPendingLoginTransactions = 5
	maxClientStateLength        = 512

	oauthResponseTypeCode       = "code"
	oauthPKCEMethod             = "S256"
	oauthGrantAuthorizationCode = "authorization_code"
	oauthGrantRefreshToken      = "refresh_token"
	oauthTokenTypeBearer        = "Bearer"
	oauthClientAuthScheme       = "Basic"
	oauthClientAuthChallenge    = `Basic realm="888 OAuth"`

	loginTransactionKeyPrefix              = "auth:v4:login:"
	authorizationCodeKeyPrefix             = "auth:v4:code:"
	loginTransactionCookiePrefix           = "oauth-tx-"
	productionLoginTransactionCookiePrefix = "__Host-oauth-tx-"
)

// ApplicationHTTPHandler coordinates application login transactions and delegated grants.
// Google identity admission and browser sessions remain owned by Service.
type ApplicationHTTPHandler struct {
	limits     RateLimiters
	service    *Service
	sessions   SessionReader
	cache      *cache.RedisClient
	registry   *config.AuthRegistry
	grants     security.DelegationStore
	production bool
}

type loginTransaction struct {
	ClientID    string   `json:"client_id"`
	Destination string   `json:"destination"`
	ClientState string   `json:"client_state"`
	Challenge   string   `json:"challenge"`
	Scopes      []string `json:"scopes"`
	BindingHash string   `json:"binding_hash"`
}

type authorizationCode struct {
	ClientID    string   `json:"client_id"`
	UserID      string   `json:"user_id"`
	RedirectURI string   `json:"redirect_uri"`
	Challenge   string   `json:"challenge"`
	Scopes      []string `json:"scopes"`
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
	UserID       string `json:"user_id"`
}

var (
	opaquePattern    = regexp.MustCompile(fmt.Sprintf(`^[A-Za-z0-9_-]{%d}$`, base64.RawURLEncoding.EncodedLen(security.OpaqueTokenBytes)))
	challengePattern = regexp.MustCompile(fmt.Sprintf(`^[A-Za-z0-9_-]{%d}$`, base64.RawURLEncoding.EncodedLen(sha256.Size)))
	verifierPattern  = regexp.MustCompile(`^[A-Za-z0-9._~-]{43,128}$`)
)

// ConfigureApplications installs the registry-driven protocol while retaining existing resource handlers.
func (h *HTTPHandler) ConfigureApplications(service *Service, client *cache.RedisClient) {
	h.applications = &ApplicationHTTPHandler{
		service:    service,
		sessions:   h.sessions,
		cache:      client,
		registry:   h.cfg.GetOAuth().Registry,
		grants:     security.DelegationStore{Cache: client},
		production: h.production,
	}
}

func transactionKey(state string) string {
	return loginTransactionKeyPrefix + security.HashOpaqueToken(state)
}

func codeKey(code string) string {
	return authorizationCodeKeyPrefix + security.HashOpaqueToken(code)
}

func (h *ApplicationHTTPHandler) cookiePrefix() string {
	if h.production {
		return productionLoginTransactionCookiePrefix
	}

	return loginTransactionCookiePrefix
}

// Login starts a registered cookie application through browser navigation.
func (h *ApplicationHTTPHandler) Login(c *fiber.Ctx) error {
	setNoStoreHeaders(c)

	app, ok := h.registry.Application(c.Query("client_id"))
	if !ok || app.Mode != config.CookieApplication || c.Query("redirect_to") != "" {
		return h.localError(c, fiber.StatusBadRequest, "Invalid login application or request")
	}

	path := c.Query("return_to", app.DefaultReturnPath)
	destination, err := config.ResolveReturnPath(app.FrontendOrigin, path)
	if err != nil {
		return h.localError(c, fiber.StatusBadRequest, "Invalid return path")
	}

	return h.begin(c, loginTransaction{
		ClientID:    app.ID,
		Destination: destination,
	})
}

// Authorize validates a backend application before resuming account authentication.
func (h *ApplicationHTTPHandler) Authorize(c *fiber.Ctx) error {
	setNoStoreHeaders(c)

	app, ok := h.registry.Application(c.Query("client_id"))
	if !ok || app.Mode != config.CodeApplication || !slices.Contains(app.RedirectURIs, c.Query("redirect_uri")) {
		return h.localError(c, fiber.StatusBadRequest, "Invalid authorization application or callback")
	}

	tx := loginTransaction{
		ClientID:    app.ID,
		Destination: c.Query("redirect_uri"),
		ClientState: c.Query("state"),
		Challenge:   c.Query("code_challenge"),
		Scopes:      strings.Fields(c.Query("scope")),
	}
	if c.Query("response_type") != oauthResponseTypeCode ||
		c.Query("code_challenge_method") != oauthPKCEMethod ||
		!challengePattern.MatchString(tx.Challenge) ||
		tx.ClientState == "" ||
		len(tx.ClientState) > maxClientStateLength ||
		len(tx.Scopes) == 0 {
		return h.failure(c, tx, oauthErrorInvalidRequest)
	}

	seen := make(map[string]bool)
	for _, scope := range tx.Scopes {
		if !slices.Contains(app.AllowedScopes, scope) || seen[scope] {
			return h.failure(c, tx, oauthErrorInvalidScope)
		}
		seen[scope] = true
	}

	return h.begin(c, tx)
}

func (h *ApplicationHTTPHandler) begin(c *fiber.Ctx, tx loginTransaction) error {
	id := c.Cookies(security.SessionCookieName(h.production))
	if id != "" {
		session, err := h.sessions.Session(c.UserContext(), id)
		if err == nil {
			user, err := h.service.userRepo.GetByID(c.UserContext(), session.UserID)
			if errors.Is(err, identity.ErrUserNotFound) {
				h.clear(c, security.SessionCookieName(h.production))
			} else if err != nil {
				return h.failure(c, tx, oauthErrorTemporarilyUnavailable)
			} else if user == nil {
				return h.failure(c, tx, oauthErrorAccessDenied)
			} else {
				allowed, err := h.allowed(c, user)
				if err != nil {
					return h.failure(c, tx, oauthErrorTemporarilyUnavailable)
				}
				if !allowed {
					return h.failure(c, tx, oauthErrorAccessDenied)
				}
				h.cookie(c, security.SessionCookieName(h.production), id, h.service.cfg.GetSession().IdleTTLSeconds)

				return h.finish(c, tx, user.ID, false)
			}
		} else if !errors.Is(err, middleware.ErrSessionMissing) {
			return h.failure(c, tx, oauthErrorTemporarilyUnavailable)
		}
	}

	count := 0
	for name := range c.Request().Header.Cookies() {
		if strings.HasPrefix(string(name), h.cookiePrefix()) {
			count++
		}
	}

	if count >= maxPendingLoginTransactions {
		return h.localError(c, fiber.StatusTooManyRequests, "Too many pending login attempts; wait for them to expire")
	}

	login, err := h.service.StartOAuthLogin(c.UserContext())
	if err != nil {
		return h.failure(c, tx, oauthErrorTemporarilyUnavailable)
	}

	binding, err := security.NewOpaqueToken(security.OpaqueTokenBytes)
	if err != nil {
		return h.failure(c, tx, oauthErrorTemporarilyUnavailable)
	}
	tx.BindingHash = security.HashOpaqueToken(binding)
	if err := h.cache.SetValue(c.UserContext(), transactionKey(login.State), tx, h.registry.Lifetimes.Login); err != nil {
		return h.failure(c, tx, oauthErrorTemporarilyUnavailable)
	}

	h.cookie(c, h.cookiePrefix()+login.State, binding, h.registry.Lifetimes.Login)

	return c.Redirect(login.URL, fiber.StatusSeeOther)
}

// Callback consumes a browser-bound Google transaction and resumes its application.
func (h *ApplicationHTTPHandler) Callback(c *fiber.Ctx) error {
	setNoStoreHeaders(c)

	state := c.Query("state")
	if !opaquePattern.MatchString(state) {
		return h.localError(c, fiber.StatusBadRequest, "Invalid login transaction")
	}

	name := h.cookiePrefix() + state
	binding := c.Cookies(name)
	var tx loginTransaction
	if err := h.cache.GetValue(c.UserContext(), transactionKey(state), &tx); err != nil {
		if cache.IsMissing(err) {
			h.clear(c, name)

			return h.localError(c, fiber.StatusBadRequest, "Expired login transaction; start login again")
		}

		return h.localError(c, fiber.StatusServiceUnavailable, "Login storage is unavailable")
	}

	if binding == "" ||
		subtle.ConstantTimeCompare([]byte(tx.BindingHash), []byte(security.HashOpaqueToken(binding))) != 1 {
		return h.localError(c, fiber.StatusBadRequest, "Invalid login browser binding")
	}

	if err := h.cache.ConsumeValue(c.UserContext(), transactionKey(state), &tx); err != nil {
		return h.localError(c, fiber.StatusBadRequest, "Login transaction was already completed")
	}

	h.clear(c, name)
	app, ok := h.registry.Application(tx.ClientID)
	if !ok || (app.Mode == config.CodeApplication && !slices.Contains(app.RedirectURIs, tx.Destination)) {
		return h.localError(c, fiber.StatusBadRequest, "Application is no longer available")
	}

	if c.Query("error") != "" {
		if _, err := h.service.authRepo.ConsumeOAuthState(c.UserContext(), security.ToOAuthStateCacheKey(state)); err != nil {
			return h.failure(c, tx, oauthErrorTemporarilyUnavailable)
		}

		return h.failure(c, tx, oauthErrorAccessDenied)
	}

	credentials, err := h.service.VerifyOAuthLogin(
		c.UserContext(),
		c.Query("code"),
		state,
		state,
		c.Cookies(security.SessionCookieName(h.production)),
	)
	if err != nil {
		if errors.Is(err, ErrEmailNotAllowed) || errors.Is(err, ErrUnverifiedEmail) {
			return h.failure(c, tx, oauthErrorAccessDenied)
		}

		return h.failure(c, tx, oauthErrorTemporarilyUnavailable)
	}

	h.cookie(c, security.SessionCookieName(h.production), credentials.SessionID, h.service.cfg.GetSession().IdleTTLSeconds)

	return h.finish(c, tx, credentials.UserID, credentials.IsNewUser)
}

func (h *ApplicationHTTPHandler) finish(c *fiber.Ctx, tx loginTransaction, userID string, isNew bool) error {
	app, ok := h.registry.Application(tx.ClientID)
	if !ok {
		return h.localError(c, fiber.StatusBadRequest, "Application is no longer available")
	}

	if app.Mode == config.CookieApplication {
		destination := tx.Destination
		if isNew {
			onboarding, err := config.ResolveReturnPath(app.FrontendOrigin, app.OnboardingPath)
			if err != nil {
				return h.localError(c, fiber.StatusInternalServerError, "Invalid onboarding configuration")
			}

			parsed, err := url.Parse(onboarding)
			if err != nil {
				return h.localError(c, fiber.StatusInternalServerError, "Invalid onboarding destination")
			}

			original, err := url.Parse(destination)
			if err != nil {
				return h.localError(c, fiber.StatusInternalServerError, "Invalid return destination")
			}

			query := parsed.Query()
			query.Set("return_to", original.RequestURI())
			parsed.RawQuery = query.Encode()
			destination = parsed.String()
		}

		return c.Redirect(destination, fiber.StatusSeeOther)
	}

	code, err := security.NewOpaqueToken(security.OpaqueTokenBytes)
	if err != nil {
		return h.failure(c, tx, oauthErrorTemporarilyUnavailable)
	}

	record := authorizationCode{
		ClientID:    app.ID,
		UserID:      userID,
		RedirectURI: tx.Destination,
		Challenge:   tx.Challenge,
		Scopes:      tx.Scopes,
	}
	if err := h.cache.SetValue(c.UserContext(), codeKey(code), record, h.registry.Lifetimes.Code); err != nil {
		return h.failure(c, tx, oauthErrorTemporarilyUnavailable)
	}

	return h.redirectParameters(c, tx.Destination, map[string]string{
		"code":  code,
		"state": tx.ClientState,
	})
}

func (h *ApplicationHTTPHandler) authenticateClient(c *fiber.Ctx) (config.AuthApplication, bool) {
	parts := strings.Fields(c.Get(fiber.HeaderAuthorization))
	if len(parts) != 2 || !strings.EqualFold(parts[0], oauthClientAuthScheme) {
		return config.AuthApplication{}, false
	}

	decoded, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return config.AuthApplication{}, false
	}

	id, secret, ok := strings.Cut(string(decoded), ":")
	if !ok {
		return config.AuthApplication{}, false
	}

	id, err = url.QueryUnescape(id)
	if err != nil {
		return config.AuthApplication{}, false
	}

	secret, err = url.QueryUnescape(secret)
	if err != nil {
		return config.AuthApplication{}, false
	}

	app, ok := h.registry.Application(id)

	return app, ok && app.Mode == config.CodeApplication && subtle.ConstantTimeCompare([]byte(app.Secret), []byte(secret)) == 1
}

// Token exchanges codes or rotates refresh credentials for an authenticated client.
func (h *ApplicationHTTPHandler) Token(c *fiber.Ctx) error {
	setNoStoreHeaders(c)

	app, ok := h.authenticateClient(c)
	if !ok {
		if h.limits.InvalidClient != nil {
			if err := h.limits.InvalidClient(c, httplimit.ClientIP(c)); err != nil {
				return err
			}
		}
		c.Set(fiber.HeaderWWWAuthenticate, oauthClientAuthChallenge)

		return oauthError(c, fiber.StatusUnauthorized, oauthErrorInvalidClient)
	}

	if h.limits.Token != nil {
		if err := h.limits.Token(c, app.ID); err != nil {
			return err
		}
	}

	if !strings.HasPrefix(c.Get(fiber.HeaderContentType), fiber.MIMEApplicationForm) {
		return oauthError(c, fiber.StatusBadRequest, oauthErrorInvalidRequest)
	}

	switch c.FormValue("grant_type") {
	case oauthGrantAuthorizationCode:
		return h.exchange(c, app)
	case oauthGrantRefreshToken:
		return h.refresh(c, app)
	default:
		return oauthError(c, fiber.StatusBadRequest, oauthErrorUnsupportedGrantType)
	}
}

func (h *ApplicationHTTPHandler) exchange(c *fiber.Ctx, app config.AuthApplication) error {
	code := c.FormValue("code")
	verifier := c.FormValue("code_verifier")
	if !opaquePattern.MatchString(code) || !verifierPattern.MatchString(verifier) {
		return oauthError(c, fiber.StatusBadRequest, oauthErrorInvalidGrant)
	}
	var record authorizationCode
	if err := h.cache.GetValue(c.UserContext(), codeKey(code), &record); err != nil {
		if cache.IsMissing(err) {
			return oauthError(c, fiber.StatusBadRequest, oauthErrorInvalidGrant)
		}

		return oauthError(c, fiber.StatusServiceUnavailable, oauthErrorTemporarilyUnavailable)
	}

	digest := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(digest[:])
	if record.ClientID != app.ID ||
		record.RedirectURI != c.FormValue("redirect_uri") ||
		!slices.Contains(app.RedirectURIs, record.RedirectURI) ||
		subtle.ConstantTimeCompare([]byte(record.Challenge), []byte(challenge)) != 1 ||
		!scopesAllowed(record.Scopes, app.AllowedScopes) {
		return oauthError(c, fiber.StatusBadRequest, oauthErrorInvalidGrant)
	}

	if err := h.checkUser(c, record.UserID); err != nil {
		return h.userError(c, err)
	}

	id, err := security.NewOpaqueToken(security.OpaqueTokenBytes)
	if err != nil {
		return oauthError(c, fiber.StatusServiceUnavailable, oauthErrorTemporarilyUnavailable)
	}

	refresh, err := security.NewOpaqueToken(security.OpaqueTokenBytes)
	if err != nil {
		return oauthError(c, fiber.StatusServiceUnavailable, oauthErrorTemporarilyUnavailable)
	}

	grant := security.Delegation{
		ID:          id,
		UserID:      record.UserID,
		ClientID:    app.ID,
		Scopes:      record.Scopes,
		RefreshHash: security.HashOpaqueToken(refresh),
		ExpiresAt:   time.Now().Unix() + int64(h.registry.Lifetimes.Absolute),
	}
	response, err := h.tokens(grant, refresh)
	if err != nil {
		return oauthError(c, fiber.StatusServiceUnavailable, oauthErrorTemporarilyUnavailable)
	}

	payload, err := json.Marshal(record)
	if err != nil {
		return oauthError(c, fiber.StatusServiceUnavailable, oauthErrorTemporarilyUnavailable)
	}

	created, err := h.grants.Create(c.UserContext(), codeKey(code), string(payload), grant, refresh, h.registry.Lifetimes.Idle)
	if err != nil {
		return oauthError(c, fiber.StatusServiceUnavailable, oauthErrorTemporarilyUnavailable)
	}

	if !created {
		return oauthError(c, fiber.StatusBadRequest, oauthErrorInvalidGrant)
	}

	return c.JSON(response)
}

func (h *ApplicationHTTPHandler) refresh(c *fiber.Ctx, app config.AuthApplication) error {
	old := c.FormValue("refresh_token")
	if !opaquePattern.MatchString(old) {
		return oauthError(c, fiber.StatusBadRequest, oauthErrorInvalidGrant)
	}

	marker, err := h.grants.RefreshMarker(c.UserContext(), old)
	if err != nil {
		if cache.IsMissing(err) {
			return oauthError(c, fiber.StatusBadRequest, oauthErrorInvalidGrant)
		}

		return oauthError(c, fiber.StatusServiceUnavailable, oauthErrorTemporarilyUnavailable)
	}

	if marker.ClientID != app.ID || !scopesAllowed(marker.Scopes, app.AllowedScopes) {
		return oauthError(c, fiber.StatusBadRequest, oauthErrorInvalidGrant)
	}

	if err := h.checkUser(c, marker.UserID); err != nil {
		return h.userError(c, err)
	}

	replacement, err := security.NewOpaqueToken(security.OpaqueTokenBytes)
	if err != nil {
		return oauthError(c, fiber.StatusServiceUnavailable, oauthErrorTemporarilyUnavailable)
	}

	response, err := h.tokens(*marker, replacement)
	if err != nil {
		return oauthError(c, fiber.StatusServiceUnavailable, oauthErrorTemporarilyUnavailable)
	}

	_, rotated, err := h.grants.Rotate(c.UserContext(), old, replacement, *marker, h.registry.Lifetimes.Idle)
	if err != nil {
		return oauthError(c, fiber.StatusServiceUnavailable, oauthErrorTemporarilyUnavailable)
	}

	if !rotated {
		return oauthError(c, fiber.StatusBadRequest, oauthErrorInvalidGrant)
	}

	return c.JSON(response)
}

func (h *ApplicationHTTPHandler) tokens(grant security.Delegation, refresh string) (*tokenResponse, error) {
	seconds := min(h.registry.Lifetimes.Access, int(grant.ExpiresAt-time.Now().Unix()))
	if seconds <= 0 {
		return nil, errors.New("delegation expired")
	}

	token, err := security.SignDelegatedToken(grant, h.service.cfg.GetJWT().AccessTokenSecret, h.service.cfg.GetServer().Name, seconds)
	if err != nil {
		return nil, err
	}

	return &tokenResponse{
		AccessToken:  token,
		RefreshToken: refresh,
		TokenType:    oauthTokenTypeBearer,
		ExpiresIn:    seconds,
		Scope:        strings.Join(grant.Scopes, " "),
		UserID:       grant.UserID,
	}, nil
}

// Revoke revokes only delegations belonging to the authenticated application.
func (h *ApplicationHTTPHandler) Revoke(c *fiber.Ctx) error {
	setNoStoreHeaders(c)

	app, ok := h.authenticateClient(c)
	if !ok {
		if h.limits.InvalidClient != nil {
			if err := h.limits.InvalidClient(c, httplimit.ClientIP(c)); err != nil {
				return err
			}
		}
		c.Set(fiber.HeaderWWWAuthenticate, oauthClientAuthChallenge)

		return oauthError(c, fiber.StatusUnauthorized, oauthErrorInvalidClient)
	}

	if h.limits.Revoke != nil {
		if err := h.limits.Revoke(c, app.ID); err != nil {
			return err
		}
	}

	if !strings.HasPrefix(c.Get(fiber.HeaderContentType), fiber.MIMEApplicationForm) {
		return oauthError(c, fiber.StatusBadRequest, oauthErrorInvalidRequest)
	}

	token := c.FormValue("token")
	var grantID string
	if opaquePattern.MatchString(token) {
		marker, err := h.grants.RefreshMarker(c.UserContext(), token)
		if err != nil && !cache.IsMissing(err) {
			return oauthError(c, fiber.StatusServiceUnavailable, oauthErrorTemporarilyUnavailable)
		}
		if marker != nil && marker.ClientID == app.ID {
			grantID = marker.ID
		}
	} else if claims, err := security.ParseDelegatedToken(token, h.service.cfg.GetJWT().AccessTokenSecret, h.service.cfg.GetServer().Name); err == nil &&
		claims.ClientID == app.ID {
		grantID = claims.DelegationID
	}

	if grantID != "" {
		if err := h.grants.Revoke(c.UserContext(), grantID); err != nil {
			return oauthError(c, fiber.StatusServiceUnavailable, oauthErrorTemporarilyUnavailable)
		}
	}

	return c.SendStatus(fiber.StatusOK)
}

func (h *ApplicationHTTPHandler) allowed(c *fiber.Ctx, user *identity.User) (bool, error) {
	decision, err := h.service.policy.EvaluateLogin(c.UserContext(), user.Email, user.ID, user.RoleID)

	return err == nil && decision.Allowed && !decision.Blacklisted, err
}

func (h *ApplicationHTTPHandler) checkUser(c *fiber.Ctx, id string) error {
	user, err := h.service.userRepo.GetByID(c.UserContext(), id)
	if errors.Is(err, identity.ErrUserNotFound) || (err == nil && user == nil) {
		return ErrEmailNotAllowed
	}

	if err != nil {
		return err
	}

	allowed, err := h.allowed(c, user)
	if err != nil {
		return err
	}

	if !allowed {
		return ErrEmailNotAllowed
	}

	return nil
}

func (h *ApplicationHTTPHandler) userError(c *fiber.Ctx, err error) error {
	if errors.Is(err, ErrEmailNotAllowed) {
		return oauthError(c, fiber.StatusBadRequest, oauthErrorInvalidGrant)
	}

	return oauthError(c, fiber.StatusServiceUnavailable, oauthErrorTemporarilyUnavailable)
}

func scopesAllowed(scopes, allowed []string) bool {
	if len(scopes) == 0 {
		return false
	}

	for _, scope := range scopes {
		if !slices.Contains(allowed, scope) {
			return false
		}
	}

	return true
}

func (h *ApplicationHTTPHandler) failure(c *fiber.Ctx, tx loginTransaction, code string) error {
	app, ok := h.registry.Application(tx.ClientID)
	if !ok {
		return h.localError(c, fiber.StatusBadRequest, "Application is unavailable")
	}

	if app.Mode == config.CookieApplication {
		destination, err := config.ResolveReturnPath(app.FrontendOrigin, app.LoginErrorPath)
		if err != nil {
			return h.localError(c, fiber.StatusInternalServerError, "Invalid login error destination")
		}

		return h.redirectParameters(c, destination, map[string]string{"error": code})
	}

	return h.redirectParameters(c, tx.Destination, map[string]string{
		"error": code,
		"state": tx.ClientState,
	})
}

func (h *ApplicationHTTPHandler) redirectParameters(c *fiber.Ctx, destination string, values map[string]string) error {
	parsed, err := url.Parse(destination)
	if err != nil {
		return h.localError(c, fiber.StatusBadRequest, "Invalid destination")
	}

	query := parsed.Query()
	for name, value := range values {
		query.Set(name, value)
	}
	parsed.RawQuery = query.Encode()

	return c.Redirect(parsed.String(), fiber.StatusSeeOther)
}

func (h *ApplicationHTTPHandler) localError(c *fiber.Ctx, status int, message string) error {
	setNoStoreHeaders(c)

	c.Type("html", "utf-8")

	return c.Status(status).SendString("<!doctype html><title>Sign-in error</title><h1>Sign-in could not complete</h1><p>" + html.EscapeString(message) + "</p><p>Request ID: " + html.EscapeString(c.GetRespHeader(apierror.RequestIDHeader)) + "</p>")
}

func oauthError(c *fiber.Ctx, status int, code string) error {
	return c.Status(status).JSON(fiber.Map{"error": code})
}

func (h *ApplicationHTTPHandler) cookie(c *fiber.Ctx, name, value string, seconds int) {
	c.Cookie(&fiber.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		MaxAge:   seconds,
		HTTPOnly: true,
		Secure:   true,
		SameSite: httpcookie.SameSite(h.production),
	})
}

func (h *ApplicationHTTPHandler) clear(c *fiber.Ctx, name string) {
	c.Cookie(&fiber.Cookie{
		Name:     name,
		Path:     "/",
		MaxAge:   -1,
		Expires:  time.Unix(1, 0),
		HTTPOnly: true,
		Secure:   true,
		SameSite: httpcookie.SameSite(h.production),
	})
}
