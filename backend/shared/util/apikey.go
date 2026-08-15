package util

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// GenerateAPIKey creates a secure random API key with a readable prefix.
func GenerateAPIKey() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return fmt.Sprintf("bm_live_%s", hex.EncodeToString(b))
}
