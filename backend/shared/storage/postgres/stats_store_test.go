package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatsSummary_RawPath(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	_, err := db.pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS request_logs (
			id BIGSERIAL PRIMARY KEY,
			tenant_id UUID NOT NULL,
			network_id UUID,
			method TEXT NOT NULL,
			status TEXT NOT NULL,
			latency_ms INTEGER NOT NULL DEFAULT 0,
			cache_hit BOOLEAN NOT NULL DEFAULT false,
			bytes_in INTEGER NOT NULL DEFAULT 0,
			request_id TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		CREATE TABLE IF NOT EXISTS blockchain_configs (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			name TEXT NOT NULL,
			rpc_endpoint_1 TEXT NOT NULL,
			chain_id TEXT,
			enabled BOOLEAN NOT NULL DEFAULT true,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	require.NoError(t, err)

	now := time.Now()
	for i := 0; i < 5; i++ {
		_, err := db.pool.Exec(ctx,
			`INSERT INTO request_logs (tenant_id, method, status, latency_ms, cache_hit, created_at)
			 VALUES ($1, $2, $3, $4, $5, $6)`,
			"tenant-stats", "eth_chainId", "success", 10+i, true, now.Add(-time.Duration(i)*time.Second),
		)
		require.NoError(t, err)
	}

	// Recent from time forces raw path
	summary, err := db.GetStatsSummary(ctx, now.Add(-2*time.Minute), "1h")
	require.NoError(t, err)
	assert.Equal(t, "1h", summary.Range)
	assert.GreaterOrEqual(t, summary.Totals.Requests, int64(5))
	assert.GreaterOrEqual(t, summary.Totals.Success, int64(5))
	assert.GreaterOrEqual(t, summary.Totals.CacheHits, int64(5))
	assert.Len(t, summary.TopMethods, 1)
	assert.Equal(t, "eth_chainId", summary.TopMethods[0].Name)
}

func TestStatsSummary_RollupPath(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	_, err := db.pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS request_logs (
			id BIGSERIAL PRIMARY KEY,
			tenant_id UUID NOT NULL,
			network_id UUID,
			method TEXT NOT NULL,
			status TEXT NOT NULL,
			latency_ms INTEGER NOT NULL DEFAULT 0,
			cache_hit BOOLEAN NOT NULL DEFAULT false,
			bytes_in INTEGER NOT NULL DEFAULT 0,
			request_id TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		CREATE MATERIALIZED VIEW IF NOT EXISTS request_logs_rollup_1m AS
		SELECT date_trunc('minute', created_at) AS bucket, COALESCE(network_id::text, '') AS network_id,
		       method, status, cache_hit, COUNT(*) AS requests,
		       COUNT(*) FILTER (WHERE status <> 'success') AS errors,
		       COUNT(*) FILTER (WHERE cache_hit) AS cache_hits,
		       COALESCE(AVG(latency_ms)::float8, 0) AS avg_latency_ms,
		       COALESCE(percentile_cont(0.95) WITHIN GROUP (ORDER BY latency_ms)::float8, 0) AS p95_latency_ms
		FROM request_logs GROUP BY 1, 2, 3, 4, 5 WITH NO DATA
	`)
	require.NoError(t, err)

	now := time.Now()
	for i := 0; i < 3; i++ {
		_, err := db.pool.Exec(ctx,
			`INSERT INTO request_logs (tenant_id, method, status, created_at)
			 VALUES ($1, $2, $3, $4)`,
			"tenant-rollup", "eth_blockNumber", "success", now.Add(-time.Duration(i+10)*time.Minute),
		)
		require.NoError(t, err)
	}

	_, err = db.pool.Exec(ctx, `REFRESH MATERIALIZED VIEW request_logs_rollup_1m`)
	require.NoError(t, err)

	summary, err := db.GetStatsSummary(ctx, now.Add(-1*time.Hour), "24h")
	require.NoError(t, err)
	assert.Equal(t, "24h", summary.Range)
	assert.GreaterOrEqual(t, summary.Totals.Requests, int64(3))
	assert.GreaterOrEqual(t, len(summary.Series), 1)
}