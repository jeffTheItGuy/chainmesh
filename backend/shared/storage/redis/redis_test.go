package redis

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_GetSet(t *testing.T) {
	s := miniredis.RunT(t)
	c := New(s.Addr())

	ctx := context.Background()
	require.NoError(t, c.Set(ctx, "key1", "val1", time.Hour))

	val, err := c.Get(ctx, "key1")
	require.NoError(t, err)
	assert.Equal(t, "val1", val)

	_, err = c.Get(ctx, "missing")
	assert.Error(t, err) // redis.Nil
}

func TestClient_IncrExpire(t *testing.T) {
	s := miniredis.RunT(t)
	c := New(s.Addr())

	ctx := context.Background()
	n, err := c.Incr(ctx, "counter")
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)

	n2, err := c.Incr(ctx, "counter")
	require.NoError(t, err)
	assert.Equal(t, int64(2), n2)

	require.NoError(t, c.Expire(ctx, "counter", time.Second))
	s.FastForward(2 * time.Second)

	_, err = c.Get(ctx, "counter")
	assert.Error(t, err) // expired
}

func TestCheckRateLimits_DailyQuota(t *testing.T) {
	s := miniredis.RunT(t)
	c := New(s.Addr())

	ctx := context.Background()
	tenant := "tenant-daily"

	// Daily quota of 2
	for i := 0; i < 2; i++ {
		allowed, _, err := c.CheckRateLimits(ctx, tenant, 0, 0, 2)
		require.NoError(t, err)
		assert.True(t, allowed, "request %d should pass", i+1)
	}

	allowed, status, err := c.CheckRateLimits(ctx, tenant, 0, 0, 2)
	require.NoError(t, err)
	assert.False(t, allowed)
	assert.Equal(t, "daily", status.RejectedLimit)
	assert.Greater(t, status.RetryAfterSeconds, 0)
}

func TestCheckRateLimits_DisabledTier(t *testing.T) {
	s := miniredis.RunT(t)
	c := New(s.Addr())

	ctx := context.Background()
	tenant := "tenant-disabled"

	// quota=0 means disabled — should never reject for that tier
	allowed, _, err := c.CheckRateLimits(ctx, tenant, 0, 0, 0)
	require.NoError(t, err)
	assert.True(t, allowed)

	// RPS disabled but RPM enabled
	allowed, status, err := c.CheckRateLimits(ctx, tenant, 0, 1, 0)
	require.NoError(t, err)
	assert.True(t, allowed)
	assert.Equal(t, 1, status.LimitMinute)

	// Exhaust RPM
	allowed, status, err = c.CheckRateLimits(ctx, tenant, 0, 1, 0)
	require.NoError(t, err)
	assert.False(t, allowed)
	assert.Equal(t, "rpm", status.RejectedLimit)
}

func TestCheckRateLimit_Legacy(t *testing.T) {
	s := miniredis.RunT(t)
	c := New(s.Addr())

	ctx := context.Background()
	tenant := "tenant-legacy"

	allowed, err := c.CheckRateLimit(ctx, tenant, 2)
	require.NoError(t, err)
	assert.True(t, allowed)

	allowed, err = c.CheckRateLimit(ctx, tenant, 2)
	require.NoError(t, err)
	assert.True(t, allowed)

	allowed, err = c.CheckRateLimit(ctx, tenant, 2)
	require.NoError(t, err)
	assert.False(t, allowed)
}