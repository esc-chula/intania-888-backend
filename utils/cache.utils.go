package utils

import "fmt"

func ToAccessCacheKey(userId string) string {
	return fmt.Sprintf("session:%v", userId)
}

func ToRefreshCacheKey(refreshToken string) string {
	return fmt.Sprintf("refresh:%v", refreshToken)
}
