package security

import (
	"testing"
	"time"
)

func TestDelegatedTokensAreBoundToIssuerClientAndExternalAudience(t *testing.T) {
	grant := Delegation{
		ID: "grant", UserID: "player", ClientID: "games",
		Scopes: []string{"profile.read"}, ExpiresAt: time.Now().Add(time.Hour).Unix(),
	}
	token, err := SignDelegatedToken(grant, "secret", "888", 60)
	if err != nil {
		t.Fatal(err)
	}

	claims, err := ParseDelegatedToken(token, "secret", "888")
	if err != nil || claims.ClientID != "games" || claims.Subject != "player" || claims.DelegationID != "grant" {
		t.Fatalf("delegated claims: %+v, %v", claims, err)
	}

	if _, err := ParseDelegatedToken(token, "secret", "another-issuer"); err == nil {
		t.Fatal("accepted another issuer")
	}
}
