package config

type Config interface {
	GetServer() Server
	GetDb() Db
	GetCache() Cache
	GetJwt() Jwt
	GetOAuth() OAuth
	GetSwagger() Swagger
}

type Server struct {
	Name string `mapstructure:"server_name"`
	Env  string `mapstructure:"server_env"`
	Url  string `mapstructure:"server_url"`
	Host string `mapstructure:"server_host"`
	Port int    `mapstructure:"server_port"`
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
	ApiSecretKey           string `mapstructure:"jwt_api_secret_key"`
	AccessTokenSecret      string `mapstructure:"jwt_access_token_secret"`
	AccessTokenExpiration  int    `mapstructure:"jwt_access_token_expiration"`
	RefreshTokenExpiration int    `mapstructure:"jwt_refresh_token_expiration"`
}

type OAuth struct {
	ClientId     string `mapstructure:"oauth_client_id"`
	ClientSecret string `mapstructure:"oauth_client_secret"`
	RedirectUrl  string `mapstructure:"oauth_redirect_uri"`
}

type Swagger struct {
	Username string `mapstructure:"swagger_username"`
	Password string `mapstructure:"swagger_password"`
}
