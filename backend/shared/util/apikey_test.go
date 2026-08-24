package util

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateAPIKey(t *testing.T) {
	key := GenerateAPIKey()
	assert.True(t, strings.HasPrefix(key, "bm_live_"))
	assert.Len(t, key, 40) // "bm_live_" (8) + 32 hex chars
}

func TestHashAndVerifyAPIKey(t *testing.T) {
	key := GenerateAPIKey()
	hash := HashAPIKey(key)
	assert.NotEmpty(t, hash)
	assert.True(t, VerifyAPIKey(key, hash))
	assert.False(t, VerifyAPIKey(key+"tampered", hash))
}

func TestAPIKeyPrefix(t *testing.T) {
	assert.Equal(t, "bm_live_abcd", APIKeyPrefix("bm_live_abcdef"))
	assert.Equal(t, "short", APIKeyPrefix("short"))
}

func TestConstantTimeEqual(t *testing.T) {
	assert.True(t, ConstantTimeEqual("secret", "secret"))
	assert.False(t, ConstantTimeEqual("secret", "different"))
}

func TestGenerateRequestID(t *testing.T) {
	id := GenerateRequestID()
	assert.Len(t, id, 32)
}