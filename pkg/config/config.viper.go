package config

import (
	"log"
	"os"
	"strings"
	"sync"

	"github.com/spf13/viper"
)

type viperConfig struct {
	Server      `mapstructure:",squash"`
	DB          `mapstructure:",squash"`
	Cache       `mapstructure:",squash"`
	JWT         `mapstructure:",squash"`
	OAuth       `mapstructure:",squash"`
	Session     `mapstructure:",squash"`
	Swagger     `mapstructure:",squash"`
	CORS        `mapstructure:",squash"`
	DailyReward `mapstructure:",squash"`
}

var (
	once     sync.Once
	instance *viperConfig
)

// NewViperConfig loads the process-wide configuration once from defaults,
// the environment, and the environment-specific file. Development requires
// .env; production can run without bin/.env. Loading failures retain the
// existing startup behavior: decoding and binding failures terminate the process.
func NewViperConfig() *viperConfig {
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
		if appEnv == "prod" {
			v.SetDefault("server_env", "production")
		} else {
			v.SetDefault("server_env", "development")
		}
		v.SetDefault("session_idle_ttl_seconds", DefaultSessionIdleTTLSeconds)
		v.SetDefault("session_absolute_ttl_seconds", DefaultSessionAbsoluteTTLSeconds)
		v.SetDefault("daily_reward_default_amount", "300.00")

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

		registry, err := LoadAuthRegistry(v.GetString("auth_config_file"), cfg.Env, cfg.AllowOrigins, v.GetString)
		if err != nil {
			log.Fatalf("Unable to load auth registry: %v", err)
		}
		cfg.Registry = registry
		cfg.RedirectURL = registry.Google.CallbackURI
		cfg.StateExpiration = registry.Lifetimes.Login

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

// GetConfig returns the existing process-wide configuration, loading it on first use.
// Callers should obtain configuration during startup rather than request handling.
func GetConfig() Config {
	if instance == nil {
		instance = NewViperConfig()
	}

	return instance
}

// GetServer returns listener, environment, and public API URL settings.
func (c *viperConfig) GetServer() Server {
	return c.Server
}

// GetDB returns PostgreSQL connection settings.
func (c *viperConfig) GetDB() DB {
	return c.DB
}

// GetCache returns Redis connection settings.
func (c *viperConfig) GetCache() Cache {
	return c.Cache
}

// GetJWT returns the signing secret for external bearer tokens.
func (c *viperConfig) GetJWT() JWT {
	return c.JWT
}

// GetOAuth returns Google OAuth credentials, redirects, and state lifetime.
func (c *viperConfig) GetOAuth() OAuth {
	return c.OAuth
}

// GetSession returns browser session idle and absolute lifetimes in seconds.
func (c *viperConfig) GetSession() Session {
	return c.Session
}

// GetSwagger returns documentation visibility and access settings.
func (c *viperConfig) GetSwagger() Swagger {
	return c.Swagger
}

// GetCORS returns the configured browser origin allowlist.
func (c *viperConfig) GetCORS() CORS {
	return c.CORS
}

// GetDailyReward returns the default daily reward as a decimal money string.
func (c *viperConfig) GetDailyReward() DailyReward {
	return c.DailyReward
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

	bind("oauth_client_id", "OAUTH_CLIENT_ID")
	bind("oauth_client_secret", "OAUTH_CLIENT_SECRET")
	bind("auth_config_file", "AUTH_CONFIG_FILE")
	bind("session_idle_ttl_seconds", "SESSION_IDLE_TTL_SECONDS")
	bind("session_absolute_ttl_seconds", "SESSION_ABSOLUTE_TTL_SECONDS")
	bind("daily_reward_default_amount", "DAILY_REWARD_DEFAULT_AMOUNT")

	bind("swagger_enabled", "SWAGGER_ENABLED")
	bind("swagger_require_auth", "SWAGGER_REQUIRE_AUTH")
	bind("swagger_username", "SWAGGER_USERNAME")
	bind("swagger_password", "SWAGGER_PASSWORD")

	bind("cors_allow_origins", "CORS_ALLOW_ORIGINS")
}
