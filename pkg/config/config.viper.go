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
			v.SetConfigFile("/bin/.env")
		case "dev":
			v.SetConfigFile("./.env")
		default:
			panic("Error: invalid app env")
		}
		v.AutomaticEnv()

		if err := v.ReadInConfig(); err != nil {
			log.Fatalf("Error reading configs file: %s", err)
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
