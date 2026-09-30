package config

import (
	"strings"
	"testing"
)

type securityValidationConfig struct {
	server  configServer
	jwt     JWT
	oauth   OAuth
	session Session
	cors    CORS
}

type configServer struct {
	Name string
	Env  string
	URL  string
}

func (c securityValidationConfig) GetServer() Server {
	return Server{Name: c.server.Name, Env: c.server.Env, URL: c.server.URL}
}
func (c securityValidationConfig) GetDB() DB           { return DB{} }
func (c securityValidationConfig) GetCache() Cache     { return Cache{Host: "localhost", Port: 6379} }
func (c securityValidationConfig) GetJWT() JWT         { return c.jwt }
func (c securityValidationConfig) GetOAuth() OAuth     { return c.oauth }
func (c securityValidationConfig) GetSession() Session { return c.session }
func (c securityValidationConfig) GetSwagger() Swagger { return Swagger{} }
func (c securityValidationConfig) GetCORS() CORS       { return c.cors }
func (c securityValidationConfig) GetDailyReward() DailyReward {
	return DailyReward{}
}

func validSecurityConfig(env string) securityValidationConfig {
	registry := &AuthRegistry{
		Version:      1,
		Lifetimes:    AuthLifetimes{Login: 600, Code: 60, Access: 3600, Idle: 604800, Absolute: 2592000},
		Applications: []AuthApplication{{ID: "web", Mode: CookieApplication, FrontendOrigin: "http://localhost:3001", DefaultReturnPath: "/", OnboardingPath: "/register", LoginErrorPath: "/login-error"}},
	}
	registry.Google.CallbackURI = "http://localhost:8080/api/v1/auth/callback"

	return securityValidationConfig{
		server: configServer{Name: "intania", Env: env, URL: "http://api.example.test/api/v1"},
		jwt: JWT{
			AccessTokenSecret: strings.Repeat("a", 32),
		},
		oauth: OAuth{
			ClientID:     "client-id",
			ClientSecret: "client-secret",
			RedirectURL:  "http://localhost:8080/api/v1/auth/callback",
			Registry:     registry,
		},
		session: Session{
			IdleTTLSeconds:     DefaultSessionIdleTTLSeconds,
			AbsoluteTTLSeconds: DefaultSessionAbsoluteTTLSeconds,
		},
		cors: CORS{AllowOrigins: "http://localhost:3001"},
	}
}

func TestValidateSecurityAcceptsDevelopmentCookieConfiguration(t *testing.T) {
	if err := ValidateSecurity(validSecurityConfig("development")); err != nil {
		t.Fatalf("ValidateSecurity() error = %v", err)
	}
}

func TestValidateSecurityRejectsProductionInsecureConfiguration(t *testing.T) {
	cfg := validSecurityConfig("production")
	if err := ValidateSecurity(cfg); err == nil {
		t.Fatal("ValidateSecurity() accepted HTTP and insecure production cookies")
	}

	cfg = validSecurityConfig("production")
	cfg.server.URL = "https://api.example.test/api/v1"
	cfg.oauth.RedirectURL = "https://api.example.test/api/v1/auth/callback"
	cfg.oauth.Registry.Google.CallbackURI = cfg.oauth.RedirectURL
	cfg.oauth.Registry.Applications[0].FrontendOrigin = "https://frontend.example.test"
	cfg.cors.AllowOrigins = "https://frontend.example.test"
	if err := ValidateSecurity(cfg); err != nil {
		t.Fatalf("ValidateSecurity() rejected valid production configuration: %v", err)
	}
}

func TestValidateSecurityRejectsWildcardAndNonExactOrigins(t *testing.T) {
	cfg := validSecurityConfig("development")
	cfg.cors.AllowOrigins = "*"
	if err := ValidateSecurity(cfg); err == nil {
		t.Fatal("ValidateSecurity() accepted wildcard CORS origins")
	}

	cfg = validSecurityConfig("development")
	cfg.cors.AllowOrigins = "http://frontend.example.test/app"
	if err := ValidateSecurity(cfg); err == nil {
		t.Fatal("ValidateSecurity() accepted a CORS path instead of an exact origin")
	}
}

func TestValidateSecurityRequiresRegisteredFrontendOrigin(t *testing.T) {
	cfg := validSecurityConfig("development")
	cfg.oauth.Registry = &AuthRegistry{
		Version:      1,
		Lifetimes:    AuthLifetimes{Login: 600, Code: 60, Access: 3600, Idle: 604800, Absolute: 2592000},
		Applications: []AuthApplication{{ID: "web", Mode: CookieApplication, FrontendOrigin: "http://other-frontend.example.test", DefaultReturnPath: "/", OnboardingPath: "/register", LoginErrorPath: "/login-error"}},
	}
	cfg.oauth.Registry.Google.CallbackURI = "http://localhost:8080/api/v1/auth/callback"
	if err := ValidateSecurity(cfg); err == nil {
		t.Fatal("ValidateSecurity() accepted a frontend origin not present in CORS_ALLOW_ORIGINS")
	}
}
