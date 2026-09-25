package config

import (
	"log"
	"os"
	"strings"
	"sync"

	"github.com/spf13/viper"
)

type viperConfig struct {
	Server  `mapstructure:",squash"`
	Db      `mapstructure:",squash"`
	Cache   `mapstructure:",squash"`
	Jwt     `mapstructure:",squash"`
	OAuth   `mapstructure:",squash"`
	Swagger `mapstructure:",squash"`
	Cors    `mapstructure:",squash"`
}

var (
	once     sync.Once
	instance Config
)

func NewViperConfig() Config {
	once.Do(func() {
		appEnv := getEnv()
		v := viper.New()

		switch appEnv {
		case "prod":
			v.SetConfigFile("./bin/.env")
		case "dev":
			v.SetConfigFile("./.env")
		default:
			panic("Error: invalid app env")
		}

		// Swagger remains enabled for compatibility. Development is frictionless,
		// while production requires the configured Basic Auth credentials.
		v.SetDefault("swagger_enabled", true)
		v.SetDefault("swagger_require_auth", appEnv == "prod")
		v.SetDefault("oauth_state_expiration", 600)
		v.SetDefault("cookie_same_site", "lax")

		// Bind environment variables to config keys
		bindEnvVars(v)
		v.AutomaticEnv()

		if err := v.ReadInConfig(); err != nil {
			if appEnv == "prod" {
				log.Println("No config file found, using environment variables")
			} else {
				log.Fatalf("Error reading configs file: %s", err)
			}
		}

		cfg := &viperConfig{}

		err := v.Unmarshal(cfg)
		if err != nil {
			log.Fatalf("Unable to decode into struct, %v", err)
		}

		instance = cfg
	})

	return instance
}

func getEnv() string {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("APP_ENV"))) {
	case "dev", "development":
		return "dev"
	case "prod", "production":
		return "prod"
	}

	if len(os.Args) >= 2 && os.Args[1] == "dev" {
		return "dev"
	}
	return "prod"
}

func GetConfig() Config {
	if instance == nil {
		instance = NewViperConfig()
	}

	return instance
}

func (c *viperConfig) GetServer() Server {
	return c.Server
}

func (c *viperConfig) GetDb() Db {
	return c.Db
}

func (c *viperConfig) GetCache() Cache {
	return c.Cache
}

func (c *viperConfig) GetJwt() Jwt {
	return c.Jwt
}

func (c *viperConfig) GetOAuth() OAuth {
	return c.OAuth
}

func (c *viperConfig) GetSwagger() Swagger {
	return c.Swagger
}

func (c *viperConfig) GetCors() Cors {
	return c.Cors
}

func bindEnvVars(v *viper.Viper) {
	bind := func(key, env string) {
		if err := v.BindEnv(key, env); err != nil {
			log.Fatalf("Unable to bind %s: %v", env, err)
		}
	}

	bind("server_name", "SERVER_NAME")
	bind("server_env", "SERVER_ENV")
	bind("server_url", "SERVER_URL")
	bind("server_host", "SERVER_HOST")
	bind("server_port", "SERVER_PORT")
	bind("server_origin", "SERVER_ORIGIN")

	bind("db_host", "DB_HOST")
	bind("db_port", "DB_PORT")
	bind("db_user", "DB_USER")
	bind("db_pass", "DB_PASS")
	bind("db_name", "DB_NAME")
	bind("db_ssl_mode", "DB_SSL_MODE")
	bind("db_timezone", "DB_TIMEZONE")

	bind("cache_host", "CACHE_HOST")
	bind("cache_port", "CACHE_PORT")
	bind("cache_pass", "CACHE_PASS")

	bind("jwt_access_token_secret", "JWT_ACCESS_TOKEN_SECRET")
	bind("jwt_access_token_expiration", "JWT_ACCESS_TOKEN_EXPIRATION")
	bind("jwt_refresh_token_expiration", "JWT_REFRESH_TOKEN_EXPIRATION")

	bind("oauth_client_id", "OAUTH_CLIENT_ID")
	bind("oauth_client_secret", "OAUTH_CLIENT_SECRET")
	bind("oauth_redirect_uri", "OAUTH_REDIRECT_URI")
	bind("oauth_post_login_redirect_url", "OAUTH_POST_LOGIN_REDIRECT_URL")
	bind("oauth_state_expiration", "OAUTH_STATE_EXPIRATION")
	bind("cookie_same_site", "COOKIE_SAME_SITE")
	bind("cookie_secure", "COOKIE_SECURE")

	bind("swagger_enabled", "SWAGGER_ENABLED")
	bind("swagger_require_auth", "SWAGGER_REQUIRE_AUTH")
	bind("swagger_username", "SWAGGER_USERNAME")
	bind("swagger_password", "SWAGGER_PASSWORD")

	bind("cors_allow_origins", "CORS_ALLOW_ORIGINS")
}
