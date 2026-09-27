package config

type Config interface {
	GetServer() Server
	GetDb() Db
	GetCache() Cache
	GetJwt() Jwt
	GetOAuth() OAuth
	GetSession() Session
	GetSwagger() Swagger
	GetCors() Cors
}

const (
	DefaultSessionIdleTTLSeconds     = 7 * 24 * 3600
	DefaultSessionAbsoluteTTLSeconds = 30 * 24 * 3600
)

type Server struct {
	Origin string `mapstructure:"server_origin"`
	Name   string `mapstructure:"server_name"`
	Env    string `mapstructure:"server_env"`
	Url    string `mapstructure:"server_url"`
	Host   string `mapstructure:"server_host"`
	Port   int    `mapstructure:"server_port"`
}

type Cors struct {
	AllowOrigins string `mapstructure:"cors_allow_origins"`
}

type Db struct {
	Host     string `mapstructure:"db_host"`
	Port     int    `mapstructure:"db_port"`
	User     string `mapstructure:"db_user"`
	Password string `mapstructure:"db_pass"`
	Name     string `mapstructure:"db_name"`
	SSLMode  string `mapstructure:"db_ssl_mode"`
	Timezone string `mapstructure:"db_timezone"`
}

type Cache struct {
	Host     string `mapstructure:"cache_host"`
	Port     int    `mapstructure:"cache_port"`
	Password string `mapstructure:"cache_pass"`
}

type Jwt struct {
	AccessTokenSecret string `mapstructure:"jwt_access_token_secret"`
}

type OAuth struct {
	ClientId             string `mapstructure:"oauth_client_id"`
	ClientSecret         string `mapstructure:"oauth_client_secret"`
	RedirectUrl          string `mapstructure:"oauth_redirect_uri"`
	PostLoginRedirectUrl string `mapstructure:"oauth_post_login_redirect_url"`
	StateExpiration      int    `mapstructure:"oauth_state_expiration"`
}

type Session struct {
	IdleTTLSeconds     int `mapstructure:"session_idle_ttl_seconds"`
	AbsoluteTTLSeconds int `mapstructure:"session_absolute_ttl_seconds"`
}

type Swagger struct {
	Enabled     bool   `mapstructure:"swagger_enabled"`
	RequireAuth bool   `mapstructure:"swagger_require_auth"`
	Username    string `mapstructure:"swagger_username"`
	Password    string `mapstructure:"swagger_password"`
}
