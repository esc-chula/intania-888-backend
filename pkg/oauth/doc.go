// Package oauth adapts Google OAuth authorization and userinfo requests.
// Authentication supplies PKCE inputs and applies account policy; this package
// owns the provider wire record and translates provider failures into stable errors.
package oauth
