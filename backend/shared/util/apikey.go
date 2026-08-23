package util

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// GenerateAPIKey creates a secure random API key with a readable prefix.
func GenerateAPIKey() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return fmt.Sprintf("bm_live_%s", hex.EncodeToString(b))
}

// GenerateRequestID creates a random request ID for tracing.
func GenerateRequestID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

// HashAPIKey hashes an API key using bcrypt.
// API keys should never be stored in plaintext.
func HashAPIKey(key string) string {
	hash, err := bcrypt.GenerateFromPassword([]byte(key), bcrypt.DefaultCost)
	if err != nil {
		panic(err)
	}
	return string(hash)
}

// VerifyAPIKey checks a plaintext API key against a bcrypt hash.
func VerifyAPIKey(key, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(key)) == nil
}

// APIKeyPrefix returns a short visible prefix for display purposes.
func APIKeyPrefix(key string) string {
	if len(key) > 12 {
		return key[:12]
	}
	return key
}

// ConstantTimeEqual compares two strings in constant time.
func ConstantTimeEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
