package security

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

const authCacheNamespace = "auth:v2"

const policyCacheKey = "authz:v1:policy-snapshot"

// ToOAuthStateCacheKey hashes login state into the versioned one-use OAuth cache namespace.
func ToOAuthStateCacheKey(state string) string {
	return fmt.Sprintf("%s:oauth-state:%s", authCacheNamespace, HashOpaqueToken(state))
}

// ToSessionCacheKey hashes an opaque browser credential into the retained session namespace.
func ToSessionCacheKey(sessionID string) string {
	return fmt.Sprintf("auth:v3:session:%s", HashOpaqueToken(sessionID))
}

// ToPolicySnapshotCacheKey returns the shared active-policy snapshot key.
func ToPolicySnapshotCacheKey() string {
	return policyCacheKey
}

// HashOpaqueToken returns the hexadecimal SHA-256 digest used to avoid raw credentials in cache keys.
func HashOpaqueToken(token string) string {
	digest := sha256.Sum256([]byte(token))
	return hex.EncodeToString(digest[:])
}

// ToUserSessionCacheKey locates the account's active session pointer without embedding its raw ID.
func ToUserSessionCacheKey(userID string) string {
	return fmt.Sprintf("auth:v3:user-session:%s", HashOpaqueToken(userID))
}

// ToExternalTokenCacheKey locates the revocation record for an external JWT identifier.
func ToExternalTokenCacheKey(jti string) string {
	return fmt.Sprintf("auth:v3:external:%s", HashOpaqueToken(jti))
}
