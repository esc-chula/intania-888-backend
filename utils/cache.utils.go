package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

const authCacheNamespace = "auth:v1"

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

func HashOpaqueToken(token string) string {
	digest := sha256.Sum256([]byte(token))
	return hex.EncodeToString(digest[:])
}
