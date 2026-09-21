package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// TokenBytes is the entropy behind a session token (PRD 9.4: 32 random bytes hex).
const TokenBytes = 32

// NewToken returns an opaque session token. The client never handles it
// directly — it lives in an HttpOnly cookie (FR-A3).
func NewToken() (string, error) {
	b := make([]byte, TokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate session token: %w", err)
	}
	return hex.EncodeToString(b), nil
}
