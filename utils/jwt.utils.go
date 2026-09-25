package utils

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/esc-chula/intania-888-backend/internal/model"
	"github.com/esc-chula/intania-888-backend/pkg/config"
	"github.com/golang-jwt/jwt/v5"
)

func JwtParseToken(reqToken, secretKey string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(reqToken, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secretKey), nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}

	return claims, nil
}

type AccessTokenClaims struct {
	UserId    string
	SessionId string
	Role      string
	Issuer    string
	Audience  string
}

// JwtSignAccessToken is retained for source compatibility with the legacy
// service while callers migrate to JwtSignAccessTokenWithSession. Tokens made
// by this compatibility helper intentionally have no session and cannot pass
// the strict access-token validator.
func JwtSignAccessToken(userID, role, secretKey string, expiration int) (*string, error) {
	token, err := JwtSignAccessTokenWithSession(
		userID,
		role,
		"",
		secretKey,
		config.GetConfig().GetServer().Name,
		config.GetConfig().GetServer().Name,
		expiration,
	)
	if err != nil {
		return nil, err
	}
	return &token, nil
}

func JwtSignAccessTokenWithSession(userID, role, sessionID, secretKey, issuer, audience string, expiration int) (string, error) {
	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  userID,
		"sid":  sessionID,
		"exp":  time.Now().Add(time.Second * time.Duration(expiration)).Unix(),
		"iat":  time.Now().Unix(),
		"iss":  issuer,
		"aud":  audience,
		"type": "access",
		"role": role,
	})

	accessTokenString, err := accessToken.SignedString([]byte(secretKey))
	if err != nil {
		return "", err
	}

	return accessTokenString, nil
}

func JwtParseAccessToken(rawToken, secretKey, issuer, audience string) (*AccessTokenClaims, error) {
	token, err := jwt.Parse(rawToken, func(token *jwt.Token) (interface{}, error) {
		if token.Method == nil || token.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secretKey), nil
	}, jwt.WithIssuer(issuer), jwt.WithAudience(audience))
	if err != nil || !token.Valid {
		return nil, errors.New("invalid access token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("invalid access token claims")
	}

	userID, ok := claims["sub"].(string)
	if !ok || userID == "" {
		return nil, errors.New("missing access token subject")
	}
	sessionID, ok := claims["sid"].(string)
	if !ok || sessionID == "" {
		return nil, errors.New("missing access token session")
	}
	tokenType, ok := claims["type"].(string)
	if !ok || tokenType != "access" {
		return nil, errors.New("invalid access token type")
	}
	role, _ := claims["role"].(string)

	return &AccessTokenClaims{
		UserId:    userID,
		SessionId: sessionID,
		Role:      role,
		Issuer:    issuer,
		Audience:  audience,
	}, nil
}

func JwtSignRefreshToken(expiration int) (*string, error) {
	refreshToken, err := NewOpaqueToken(32)
	if err != nil {
		return nil, err
	}

	return &refreshToken, nil
}

func NewOpaqueToken(size int) (string, error) {
	bytes := make([]byte, size)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

func NewCredentials(accessToken, refreshToken string, expiresIn int32, isNewUser bool) *model.CredentialDto {
	return &model.CredentialDto{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    expiresIn,
		IsNewUser:    isNewUser,
	}
}
