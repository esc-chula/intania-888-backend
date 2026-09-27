package utils

import "testing"

func TestExternalTokenClaimsAreBoundToAudienceAndType(t *testing.T) {
	token, err := JwtSignExternalToken("user", "id", "secret", "issuer", 3600)
	if err != nil {
		t.Fatal(err)
	}

	subject, jti, err := JwtParseExternalToken(token, "secret", "issuer")
	if err != nil || subject != "user" || jti != "id" {
		t.Fatalf("claims: %s %s %v", subject, jti, err)
	}
	if _, _, err := JwtParseExternalToken(token, "secret", "other"); err == nil {
		t.Fatal("wrong issuer accepted")
	}
	if _, _, err := JwtParseExternalToken(token, "other", "issuer"); err == nil {
		t.Fatal("wrong secret accepted")
	}
}
