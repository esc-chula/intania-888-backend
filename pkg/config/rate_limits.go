package config

import (
	"fmt"
	"strings"
)

// RatePolicy combines a sliding minute allowance with an immediate burst capacity.
type RatePolicy struct {
	PerMinute int `mapstructure:"per_minute"`
	Burst     int `mapstructure:"burst"`
}

// RateLimits contains independent, process-local request budgets.
type RateLimits struct {
	Shared              RatePolicy `mapstructure:"shared"`
	Browser             RatePolicy `mapstructure:"browser"`
	Login               RatePolicy `mapstructure:"login"`
	Callback            RatePolicy `mapstructure:"callback"`
	Token               RatePolicy `mapstructure:"token"`
	Revoke              RatePolicy `mapstructure:"revoke"`
	InvalidClient       RatePolicy `mapstructure:"invalid_client"`
	InvalidExternal     RatePolicy `mapstructure:"invalid_external"`
	ExternalClient      RatePolicy `mapstructure:"external_client"`
	ExternalProfile     RatePolicy `mapstructure:"external_profile"`
	ExternalTransaction RatePolicy `mapstructure:"external_transaction"`
}

// DefaultRateLimits returns the provisional campus-network policy.
func DefaultRateLimits() RateLimits {
	return RateLimits{
		Shared:              RatePolicy{PerMinute: 60000, Burst: 2000},
		Browser:             RatePolicy{PerMinute: 300, Burst: 20},
		Login:               RatePolicy{PerMinute: 1200, Burst: 100},
		Callback:            RatePolicy{PerMinute: 1200, Burst: 100},
		Token:               RatePolicy{PerMinute: 2000, Burst: 200},
		Revoke:              RatePolicy{PerMinute: 2000, Burst: 200},
		InvalidClient:       RatePolicy{PerMinute: 30, Burst: 5},
		InvalidExternal:     RatePolicy{PerMinute: 120, Burst: 10},
		ExternalClient:      RatePolicy{PerMinute: 10000, Burst: 300},
		ExternalProfile:     RatePolicy{PerMinute: 60, Burst: 5},
		ExternalTransaction: RatePolicy{PerMinute: 10, Burst: 3},
	}
}

// Policies returns the stable policy names used by configuration and rejection logs.
func (r RateLimits) Policies() map[string]RatePolicy {
	return map[string]RatePolicy{
		"shared": r.Shared, "browser": r.Browser, "login": r.Login,
		"callback": r.Callback, "token": r.Token, "revoke": r.Revoke,
		"invalid_client": r.InvalidClient, "invalid_external": r.InvalidExternal,
		"external_client": r.ExternalClient, "external_profile": r.ExternalProfile,
		"external_transaction": r.ExternalTransaction,
	}
}

// Validate rejects disabled or invalid policies rather than silently dropping protection.
func (r RateLimits) Validate() error {
	for name, policy := range r.Policies() {
		if policy.PerMinute <= 0 || policy.Burst <= 0 {
			return fmt.Errorf("RATE_LIMIT_%s_PER_MINUTE and RATE_LIMIT_%s_BURST must be positive", strings.ToUpper(name), strings.ToUpper(name))
		}
	}
	return nil
}
