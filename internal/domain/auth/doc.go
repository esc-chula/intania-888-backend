// Package auth coordinates Google OAuth admission, opaque browser sessions, and
// legacy external tokens. Services own protocol and admission rules; HTTP and
// Redis adapters own cookies, transport records, and cache encoding.
package auth
