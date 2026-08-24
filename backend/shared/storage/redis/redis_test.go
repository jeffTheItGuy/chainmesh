package redis

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckRateLimits_LuaScriptRace(t *testing.T) {
	s := miniredis.RunT(t)
	client := New(s.Addr())

	const quota = 50
	const workers = 100

	var passed atomic.Int64
	var rejected atomic.Int64
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			allowed, _, err := client.CheckRateLimits(context.Background(), "tenant-lua-race", 0, quota, 0)
			require.NoError(t, err)
			if allowed {
				passed.Add(1)
			} else {
				rejected.Add(1)
			}
		}()
	}
	wg.Wait()

	assert.Equal(t, int64(quota), passed.Load(), "exactly quota should pass")
	assert.Equal(t, int64(workers-quota), rejected.Load(), "remainder should be rejected")
}

func TestCheckRateLimits_KeyExpiration(t *testing.T) {
	s := miniredis.RunT(t)
	client := New(s.Addr())

	tenant := "tenant-expire"

	// Exhaust the 1 RPS bucket
	allowed1, _, err := client.CheckRateLimits(context.Background(), tenant, 1, 0, 0)
	require.NoError(t, err)
	assert.True(t, allowed1, "first request should pass")

	allowed2, _, err := client.CheckRateLimits(context.Background(), tenant, 1, 0, 0)
	require.NoError(t, err)
	assert.False(t, allowed2, "second request should be rejected")

	// Fast-forward past the 2-second TTL hardcoded for RPS keys
	s.FastForward(3 * time.Second)

	allowed3, _, err := client.CheckRateLimits(context.Background(), tenant, 1, 0, 0)
	require.NoError(t, err)
	assert.True(t, allowed3, "request after expiry should pass again")
}
