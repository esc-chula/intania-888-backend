package security

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/esc-chula/intania-888-backend/pkg/cache"
)

const (
	delegatedTokenKind  = "delegated"
	delegationKeyPrefix = "auth:v4:delegation:"
	refreshKeyPrefix    = "auth:v4:refresh:"
)

// Delegation authorizes one application to act for one account.
type Delegation struct {
	ID          string   `json:"id"`
	UserID      string   `json:"user_id"`
	ClientID    string   `json:"client_id"`
	Scopes      []string `json:"scopes"`
	RefreshHash string   `json:"refresh_hash"`
	ExpiresAt   int64    `json:"expires_at"`
}

// DelegatedClaims bind a signed credential to its account, application and grant.
type DelegatedClaims struct {
	jwt.RegisteredClaims
	Kind         string   `json:"type"`
	ClientID     string   `json:"client_id"`
	DelegationID string   `json:"delegation_id"`
	Scopes       []string `json:"scopes"`
}

// SignDelegatedToken signs a scoped external credential bounded by grant expiry.
func SignDelegatedToken(grant Delegation, secret, issuer string, seconds int) (string, error) {
	now := time.Now()
	expiry := min(now.Unix()+int64(seconds), grant.ExpiresAt)
	jti, err := NewOpaqueToken(OpaqueTokenBytes)
	if err != nil {
		return "", err
	}

	claims := DelegatedClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuer,
			Subject:   grant.UserID,
			ID:        jti,
			Audience:  jwt.ClaimStrings{issuer + ":external"},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(time.Unix(expiry, 0)),
		},
		Kind:         delegatedTokenKind,
		ClientID:     grant.ClientID,
		DelegationID: grant.ID,
		Scopes:       grant.Scopes,
	}

	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
}

// ParseDelegatedToken verifies the delegated kind, issuer, audience and expiry.
func ParseDelegatedToken(raw, secret, issuer string) (*DelegatedClaims, error) {
	claims := &DelegatedClaims{}
	_, err := jwt.ParseWithClaims(
		raw,
		claims,
		func(token *jwt.Token) (interface{}, error) {
			return []byte(secret), nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(issuer),
		jwt.WithAudience(issuer+":external"),
		jwt.WithExpirationRequired(),
	)
	if err != nil ||
		claims.Kind != delegatedTokenKind ||
		claims.Subject == "" ||
		claims.ClientID == "" ||
		claims.DelegationID == "" {
		return nil, errors.New("invalid delegated token")
	}

	return claims, nil
}

// DelegationKey hashes a grant identifier into the delegation namespace.
func DelegationKey(id string) string {
	return delegationKeyPrefix + HashOpaqueToken(id)
}

// RefreshKey hashes a refresh credential into the replay-detection namespace.
func RefreshKey(token string) string {
	return refreshKeyPrefix + HashOpaqueToken(token)
}

// DelegationStore retains consumed refresh markers until absolute expiry to detect replay.
type DelegationStore struct{ Cache *cache.RedisClient }

// Read renews an active grant within its absolute lifetime.
func (s DelegationStore) Read(ctx context.Context, id string, idle int) (*Delegation, error) {
	var grant Delegation
	if err := s.Cache.ReadAndRenewSession(ctx, DelegationKey(id), time.Now().Unix(), idle, &grant); err != nil {
		return nil, err
	}

	return &grant, nil
}

// Create consumes an unchanged authorization code and creates its grant together.
func (s DelegationStore) Create(ctx context.Context, codeKey, expected string, grant Delegation, refresh string, idle int) (bool, error) {
	payload, err := json.Marshal(grant)
	if err != nil {
		return false, err
	}

	remaining := grant.ExpiresAt - time.Now().Unix()
	result, err := s.Cache.EvalText(ctx, `
if redis.call('GET', KEYS[1]) ~= ARGV[1] then return 'missing' end
redis.call('DEL', KEYS[1])
redis.call('SET', KEYS[2], ARGV[2], 'EX', ARGV[3])
redis.call('SET', KEYS[3], ARGV[2], 'EX', ARGV[4])
return 'ok'`, []string{codeKey, DelegationKey(grant.ID), RefreshKey(refresh)}, expected, string(payload), min(int64(idle), remaining), remaining)

	return result == "ok", err
}

// RefreshMarker finds the client and grant bound to a current or consumed credential.
func (s DelegationStore) RefreshMarker(ctx context.Context, token string) (*Delegation, error) {
	var marker Delegation
	if err := s.Cache.GetValue(ctx, RefreshKey(token), &marker); err != nil {
		return nil, err
	}

	return &marker, nil
}

// Rotate invalidates the entire grant if an already consumed refresh token is reused.
func (s DelegationStore) Rotate(ctx context.Context, old, replacement string, marker Delegation, idle int) (*Delegation, bool, error) {
	marker.RefreshHash = HashOpaqueToken(replacement)
	payload, err := json.Marshal(marker)
	if err != nil {
		return nil, false, err
	}

	remaining := marker.ExpiresAt - time.Now().Unix()
	if remaining <= 0 {
		return nil, false, nil
	}

	result, err := s.Cache.EvalText(ctx, `
local raw = redis.call('GET', KEYS[1])
if not raw then return 'missing' end
local grant = cjson.decode(raw)
if grant.refresh_hash ~= ARGV[1] then redis.call('DEL', KEYS[1]); return 'replay' end
redis.call('SET', KEYS[1], ARGV[2], 'EX', ARGV[3])
redis.call('SET', KEYS[2], ARGV[2], 'EX', ARGV[4])
return 'ok'`, []string{DelegationKey(marker.ID), RefreshKey(replacement)}, HashOpaqueToken(old), string(payload), min(int64(idle), remaining), remaining)

	return &marker, result == "ok", err
}

// Revoke removes an active grant without revealing whether it existed.
func (s DelegationStore) Revoke(ctx context.Context, id string) error {
	return s.Cache.DeleteValue(ctx, DelegationKey(id))
}
