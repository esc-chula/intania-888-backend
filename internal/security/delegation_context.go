package security

import "context"

type delegatedContextKey struct{}

type verifiedDelegatedToken struct {
	raw    string
	secret string
	issuer string
	claims *DelegatedClaims
}

// ContextWithDelegatedToken verifies and caches a credential for this request only.
// Invalid credentials never install cached identity state.
func ContextWithDelegatedToken(ctx context.Context, raw, secret, issuer string) (context.Context, *DelegatedClaims, error) {
	claims, err := ParseDelegatedToken(raw, secret, issuer)
	if err != nil {
		return ctx, nil, err
	}
	cached := verifiedDelegatedToken{raw: raw, secret: secret, issuer: issuer, claims: claims}
	return context.WithValue(ctx, delegatedContextKey{}, cached), claims, nil
}

// ParseDelegatedTokenForContext reuses this request's verified credential only
// when the token, verification secret and issuer all match.
func ParseDelegatedTokenForContext(ctx context.Context, raw, secret, issuer string) (*DelegatedClaims, error) {
	cached, ok := ctx.Value(delegatedContextKey{}).(verifiedDelegatedToken)
	if ok && cached.raw == raw && cached.secret == secret && cached.issuer == issuer {
		return cached.claims, nil
	}
	return ParseDelegatedToken(raw, secret, issuer)
}
