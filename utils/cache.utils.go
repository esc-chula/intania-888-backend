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
	return fmt.Sprintf("%s:session:%s", authCacheNamespace, sessionID)
}

func ToRefreshCacheKey(refreshToken string) string {
	return ToRefreshHashCacheKey(HashOpaqueToken(refreshToken))
}

func ToRefreshHashCacheKey(refreshTokenHash string) string {
	return fmt.Sprintf("%s:refresh:%s", authCacheNamespace, refreshTokenHash)
}

func ToPolicySnapshotCacheKey() string {
	return policyCacheKey
}

func HashOpaqueToken(token string) string {
	digest := sha256.Sum256([]byte(token))
	return hex.EncodeToString(digest[:])
}
