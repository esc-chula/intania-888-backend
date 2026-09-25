package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

const authCacheNamespace = "auth:v1"

// ToAccessCacheKey is retained while the legacy handler is being removed.
// New session code uses ToSessionCacheKey with a server-generated session ID.
func ToAccessCacheKey(userID string) string {
	return ToSessionCacheKey(userID)
}

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
