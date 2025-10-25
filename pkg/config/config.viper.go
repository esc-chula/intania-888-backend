package config

import (
	"log"
	"os"
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
	if len(os.Args) >= 2 {
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
	v.BindEnv("server_name", "SERVER_NAME")
	v.BindEnv("server_env", "SERVER_ENV")
	v.BindEnv("server_host", "SERVER_HOST")
	v.BindEnv("server_port", "SERVER_PORT")
	v.BindEnv("server_origin", "SERVER_ORIGIN")

	v.BindEnv("db_host", "DB_HOST")
	v.BindEnv("db_port", "DB_PORT")
	v.BindEnv("db_user", "DB_USER")
	v.BindEnv("db_pass", "DB_PASS")
	v.BindEnv("db_name", "DB_NAME")
	v.BindEnv("db_ssl_mode", "DB_SSL_MODE")
	v.BindEnv("db_timezone", "DB_TIMEZONE")

	v.BindEnv("cache_host", "CACHE_HOST")
	v.BindEnv("cache_port", "CACHE_PORT")
	v.BindEnv("cache_pass", "CACHE_PASS")

	v.BindEnv("jwt_access_token_secret", "JWT_ACCESS_TOKEN_SECRET")
	v.BindEnv("jwt_access_token_expiration", "JWT_ACCESS_TOKEN_EXPIRATION")
	v.BindEnv("jwt_refresh_token_expiration", "JWT_REFRESH_TOKEN_EXPIRATION")

	v.BindEnv("oauth_client_id", "OAUTH_CLIENT_ID")
	v.BindEnv("oauth_client_secret", "OAUTH_CLIENT_SECRET")
	v.BindEnv("oauth_redirect_uri", "OAUTH_REDIRECT_URI")
	v.BindEnv("oauth_frontend_url", "OAUTH_FRONTEND_URL")

	v.BindEnv("swagger_username", "SWAGGER_USERNAME")
	v.BindEnv("swagger_password", "SWAGGER_PASSWORD")

	v.BindEnv("cors_allow_origins", "CORS_ALLOW_ORIGINS")
}
