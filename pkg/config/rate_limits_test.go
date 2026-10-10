package config

import (
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestRateLimitsEnvironmentAndDefaults(t *testing.T) {
	t.Setenv("RATE_LIMIT_EXTERNAL_TRANSACTION_PER_MINUTE", "17")
	t.Setenv("RATE_LIMIT_SHARED_BURST", "1500")
	t.Setenv("SERVER_CLIENT_IP_MODE", "cloud_run")
	v := viper.New()
	setRateLimitDefaults(v)
	bindEnvVars(v)
	var cfg viperConfig
	if err := v.Unmarshal(&cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.ExternalTransaction.PerMinute != 17 || cfg.ExternalTransaction.Burst != 3 {
		t.Fatalf("transaction policy = %+v", cfg.ExternalTransaction)
	}
	if cfg.Shared.PerMinute != 60000 || cfg.Shared.Burst != 1500 || cfg.ClientIPMode != "cloud_run" {
		t.Fatalf("shared = %+v, mode = %q", cfg.Shared, cfg.ClientIPMode)
	}
	if err := cfg.GetRateLimits().Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestRateLimitsRejectInvalidOverrides(t *testing.T) {
	for name := range DefaultRateLimits().Policies() {
		for _, field := range []string{"per_minute", "burst"} {
			t.Run(name+"/"+field, func(t *testing.T) {
				v := viper.New()
				setRateLimitDefaults(v)
				v.Set("rate_limits."+name+"."+field, 0)
				var cfg viperConfig
				if err := v.Unmarshal(&cfg); err != nil {
					t.Fatal(err)
				}
				if err := cfg.GetRateLimits().Validate(); err == nil {
					t.Fatal("expected invalid policy error")
				}
			})
		}
	}
}

func TestRateLimitDotenvOverridesAndEnvironmentPrecedence(t *testing.T) {
	t.Setenv("RATE_LIMIT_BROWSER_BURST", "7")
	v := viper.New()
	setRateLimitDefaults(v)
	bindEnvVars(v)
	v.SetConfigType("env")
	if err := v.ReadConfig(strings.NewReader("RATE_LIMIT_BROWSER_PER_MINUTE=123\nRATE_LIMIT_BROWSER_BURST=4\n")); err != nil {
		t.Fatal(err)
	}
	readRateLimitFileOverrides(v)
	var cfg viperConfig
	if err := v.Unmarshal(&cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Browser.PerMinute != 123 || cfg.Browser.Burst != 7 {
		t.Fatalf("dotenv/environment policy = %+v", cfg.Browser)
	}
}
