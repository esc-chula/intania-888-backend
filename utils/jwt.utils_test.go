package utils

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestAccessTokenClaimsRequireSessionAndValidateIssuerAudienceAndType(t *testing.T) {
	token, err := JwtSignAccessTokenWithSession("user", "USER", "session", "secret", "issuer", "audience", 300)
	if err != nil {
		t.Fatalf("JwtSignAccessTokenWithSession() error = %v", err)
	}
	claims, err := JwtParseAccessToken(token, "secret", "issuer", "audience")
	if err != nil {
		t.Fatalf("JwtParseAccessToken() error = %v", err)
	}
	if claims.UserId != "user" || claims.SessionId != "session" {
		t.Fatalf("claims = %#v, want user/session claims", claims)
	}

	if _, err := JwtParseAccessToken(token, "secret", "wrong-issuer", "audience"); err == nil {
		t.Fatal("token with wrong issuer was accepted")
	}
	if _, err := JwtParseAccessToken(token, "secret", "issuer", "wrong-audience"); err == nil {
		t.Fatal("token with wrong audience was accepted")
	}
}

func TestAccessTokensWithoutSessionOrWithWrongTypeAreRejected(t *testing.T) {
	legacy := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  "user",
		"exp":  time.Now().Add(time.Minute).Unix(),
		"iss":  "issuer",
		"aud":  "audience",
		"type": "access",
	})
	legacyToken, err := legacy.SignedString([]byte("secret"))
	if err != nil {
		t.Fatalf("sign legacy token: %v", err)
	}
	if _, err := JwtParseAccessToken(legacyToken, "secret", "issuer", "audience"); err == nil {
		t.Fatal("legacy token without sid was accepted")
	}

	wrongType := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  "user",
		"sid":  "session",
		"exp":  time.Now().Add(time.Minute).Unix(),
		"iss":  "issuer",
		"aud":  "audience",
		"type": "refresh",
	})
	wrongTypeToken, err := wrongType.SignedString([]byte("secret"))
	if err != nil {
		t.Fatalf("sign wrong-type token: %v", err)
	}
	if _, err := JwtParseAccessToken(wrongTypeToken, "secret", "issuer", "audience"); err == nil {
		t.Fatal("wrong-type token was accepted")
	}
}
