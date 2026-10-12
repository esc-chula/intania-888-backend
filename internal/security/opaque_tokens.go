package security

import (
	"crypto/rand"
	"encoding/base64"
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
