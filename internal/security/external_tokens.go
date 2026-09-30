package security

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// OpaqueTokenBytes is the random input size for authentication credentials.
const OpaqueTokenBytes = 32

// NewOpaqueToken encodes size cryptographically random bytes as unpadded URL-safe base64.
func NewOpaqueToken(size int) (string, error) {
	bytes := make([]byte, size)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

// JWTSignExternalToken signs a short-lived HS256 external credential with subject, issuer and external audience.
// External tokens are deliberately separate from browser sessions.
func JWTSignExternalToken(subject, jti, secret, issuer string, seconds int) (string, error) {
	if subject == "" || jti == "" || secret == "" || issuer == "" || seconds <= 0 {
		return "", errors.New("invalid external token parameters")
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  subject,
		"jti":  jti,
		"iss":  issuer,
		"aud":  issuer + ":external",
		"type": "external",
		"iat":  time.Now().Unix(),
		"exp":  time.Now().Add(time.Duration(seconds) * time.Second).Unix(),
	})

	return token.SignedString([]byte(secret))
}

// JWTParseExternalToken verifies signature, issuer, audience, expiry and external kind, returning subject and token ID.
func JWTParseExternalToken(raw, secret, issuer string) (string, string, error) {
	token, err := jwt.Parse(
		raw, func(t *jwt.Token) (interface{}, error) {
			if t.Method == nil || t.Method.Alg() != jwt.SigningMethodHS256.Alg() {
				return nil, errors.New("invalid signing method")
			}
			return []byte(secret), nil
		},
		jwt.WithIssuer(issuer),
		jwt.WithAudience(issuer+":external"),
		jwt.WithExpirationRequired(),
	)
	if err != nil || !token.Valid {
		return "", "", errors.New("invalid external token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", "", errors.New("invalid external claims")
	}

	subject, _ := claims["sub"].(string)
	jti, _ := claims["jti"].(string)
	kind, _ := claims["type"].(string)
	if subject == "" || jti == "" || kind != "external" {
		return "", "", errors.New("invalid external claims")
	}

	return subject, jti, nil
}
