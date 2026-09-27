package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

const authCacheNamespace = "auth:v2"

const policyCacheKey = "authz:v1:policy-snapshot"

func ToOAuthStateCacheKey(state string) string {
	return fmt.Sprintf("%s:oauth-state:%s", authCacheNamespace, HashOpaqueToken(state))
}

func ToSessionCacheKey(sessionID string) string {
	return fmt.Sprintf("auth:v3:session:%s", HashOpaqueToken(sessionID))
}

func ToPolicySnapshotCacheKey() string {
	return policyCacheKey
}

func HashOpaqueToken(token string) string {
	digest := sha256.Sum256([]byte(token))
	return hex.EncodeToString(digest[:])
}

func ToUserSessionCacheKey(userID string) string {
	return fmt.Sprintf("auth:v3:user-session:%s", HashOpaqueToken(userID))
}

func ToExternalTokenCacheKey(jti string) string {
	return fmt.Sprintf("auth:v3:external:%s", HashOpaqueToken(jti))
}
