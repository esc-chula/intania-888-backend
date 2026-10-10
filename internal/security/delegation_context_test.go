package security

import (
	"context"
	"testing"
	"time"
)

func TestDelegatedCredentialCacheIsRequestAndVerifierBound(t *testing.T) {
	token, err := SignDelegatedToken(Delegation{
		ID: "grant", UserID: "user", ClientID: "games", ExpiresAt: time.Now().Add(time.Hour).Unix(),
	}, "secret", "888", 60)
	if err != nil {
		t.Fatal(err)
	}
	original := context.Background()
	ctx, claims, err := ContextWithDelegatedToken(original, token, "secret", "888")
	if err != nil {
		t.Fatal(err)
	}
	cached, err := ParseDelegatedTokenForContext(ctx, token, "secret", "888")
	if err != nil || cached != claims {
		t.Fatalf("cache reuse = %p/%p, %v", cached, claims, err)
	}
	for _, tc := range []struct{ raw, secret, issuer string }{
		{token, "wrong", "888"}, {token, "secret", "other"}, {"invalid", "secret", "888"},
	} {
		if _, err := ParseDelegatedTokenForContext(ctx, tc.raw, tc.secret, tc.issuer); err == nil {
			t.Fatal("cached credential bypassed changed verification inputs")
		}
	}
	separate, err := ParseDelegatedTokenForContext(original, token, "secret", "888")
	if err != nil || separate == claims {
		t.Fatal("cache leaked into a separate request")
	}
	failed, _, err := ContextWithDelegatedToken(original, "invalid", "secret", "888")
	if err == nil || failed != original {
		t.Fatal("invalid credential installed cached identity")
	}
}
