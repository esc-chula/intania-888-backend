package config

// Config provides typed application settings to services and infrastructure adapters.
// Getters return value snapshots; loading and security validation are separate steps.
type Config interface {
	// GetRateLimits returns process-local minute and burst budgets.
	GetRateLimits() RateLimits
	// GetServer returns listener, environment, and public API URL settings.
	GetServer() Server
	// GetDB returns PostgreSQL connection settings.
	GetDB() DB
	// GetCache returns Redis connection settings.
	GetCache() Cache
	// GetJWT returns the signing secret for external bearer tokens.
	GetJWT() JWT
	// GetOAuth returns Google OAuth credentials, redirects, and state lifetime.
	GetOAuth() OAuth
	// GetSession returns browser session idle and absolute lifetimes in seconds.
	GetSession() Session
	// GetSwagger returns documentation visibility and access settings.
	GetSwagger() Swagger
	// GetCORS returns the configured browser origin allowlist.
	GetCORS() CORS
	// GetDailyReward returns the default daily reward as a decimal money string.
	GetDailyReward() DailyReward
	// GetTeamCoin returns the fixed team-coin award per won match vote as a decimal money string.
	GetTeamCoin() TeamCoin
}

const (
	// DefaultSessionIdleTTLSeconds sets the inactivity limit to seven days.
	DefaultSessionIdleTTLSeconds = 7 * 24 * 3600
	// DefaultSessionAbsoluteTTLSeconds caps a browser session at thirty days.
	DefaultSessionAbsoluteTTLSeconds = 30 * 24 * 3600
)

// Server contains the listener and public application URL configuration.
type Server struct {
	ClientIPMode string `mapstructure:"server_client_ip_mode"`
	Origin       string `mapstructure:"server_origin"`
	Name         string `mapstructure:"server_name"`
	Env          string `mapstructure:"server_env"`
	URL          string `mapstructure:"server_url"`
	Host         string `mapstructure:"server_host"`
	Port         int    `mapstructure:"server_port"`
}

// CORS contains the comma-separated exact origins allowed for browser requests.
type CORS struct {
	AllowOrigins string `mapstructure:"cors_allow_origins"`
}

// DB contains PostgreSQL connection credentials and session settings.
type DB struct {
	Host     string `mapstructure:"db_host"`
	Port     int    `mapstructure:"db_port"`
	User     string `mapstructure:"db_user"`
	Password string `mapstructure:"db_pass"`
	Name     string `mapstructure:"db_name"`
	SSLMode  string `mapstructure:"db_ssl_mode"`
	Timezone string `mapstructure:"db_timezone"`
}

// Cache contains Redis connection credentials.
type Cache struct {
	Host     string `mapstructure:"cache_host"`
	Port     int    `mapstructure:"cache_port"`
	Password string `mapstructure:"cache_pass"`
}

// JWT contains the signing secret used by external bearer authentication.
type JWT struct {
	AccessTokenSecret string `mapstructure:"jwt_access_token_secret"`
}

// OAuth contains upstream Google credentials and registered application settings.
type OAuth struct {
	ClientID     string        `mapstructure:"oauth_client_id"`
	ClientSecret string        `mapstructure:"oauth_client_secret"`
	Registry     *AuthRegistry `mapstructure:"-"`
}

// Session contains browser session lifetimes measured in seconds.
// Idle activity can extend a session only within its absolute lifetime.
type Session struct {
	IdleTTLSeconds     int `mapstructure:"session_idle_ttl_seconds"`
	AbsoluteTTLSeconds int `mapstructure:"session_absolute_ttl_seconds"`
}

// DailyReward contains the default daily reward amount as a decimal money string.
type DailyReward struct {
	DefaultAmount string `mapstructure:"daily_reward_default_amount"`
}

// TeamCoin contains the fixed award a color earns for each match it wins by vote,
// as a decimal money string.
type TeamCoin struct {
	PerMatchWin string `mapstructure:"team_coin_per_match_win"`
}

// Swagger controls documentation routes and their optional Basic authentication.
type Swagger struct {
	Enabled     bool   `mapstructure:"swagger_enabled"`
	RequireAuth bool   `mapstructure:"swagger_require_auth"`
	Username    string `mapstructure:"swagger_username"`
	Password    string `mapstructure:"swagger_password"`
}

// SwaggerApplicationID returns the cookie application registered for this API origin.
func (o OAuth) SwaggerApplicationID(apiOrigin string) string {
	if o.Registry == nil {
		return ""
	}

	app, ok := o.Registry.Application("intania-888-swagger")
	if !ok || app.Mode != CookieApplication || app.FrontendOrigin != apiOrigin ||
		app.DefaultReturnPath != "/swagger/index.html" ||
		app.OnboardingPath != "/swagger/index.html" ||
		app.LoginErrorPath != "/swagger/index.html" {
		return ""
	}

	return app.ID
}
