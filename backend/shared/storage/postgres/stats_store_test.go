package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const statsTenantID = "770e8400-e29b-41d4-a716-446655440001"
const statsNetID = "660e8400-e29b-41d4-a716-446655440001"

func TestStatsSummary_RawPath_Accuracy(t *testing.T) {
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
	rpc_endpoint_2 TEXT,
	chain_id TEXT,
	enabled BOOLEAN NOT NULL DEFAULT true,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE blockchain_configs ADD COLUMN IF NOT EXISTS rpc_endpoint_2 TEXT;
`)
	require.NoError(t, err)

	// Clean all request_logs and blockchain_configs to avoid interference from other tests
	_, err = db.pool.Exec(ctx, `TRUNCATE request_logs, blockchain_configs CASCADE`)
	require.NoError(t, err)

	// Seed a named network so JOIN resolves correctly
	_, err = db.pool.Exec(ctx,
		`INSERT INTO blockchain_configs (id, name, rpc_endpoint_1, enabled) VALUES ($1, 'TestNet', 'http://1.1.1.1:8545', true)`,
		statsNetID,
	)
	require.NoError(t, err)

	now := time.Now().Truncate(time.Second)

	_, err = db.pool.Exec(ctx, `
		INSERT INTO request_logs (tenant_id, network_id, method, status, latency_ms, cache_hit, bytes_in, created_at) VALUES
		($1, $2, 'eth_chainId',       'success',         10, true,  100, $3),
		($1, $2, 'eth_chainId',       'success',         20, true,  100, $3),
		($1, $2, 'eth_blockNumber',   'success',         30, true,  100, $3),
		($1, $2, 'eth_blockNumber',   'success',         40, false, 100, $3),
		($1, $2, 'eth_blockNumber',   'success',         50, false, 100, $3),
		($1, $2, 'eth_call',          'rpc_error',       60, false, 100, $3),
		($1, $2, 'eth_call',          'rpc_error',       70, false, 100, $3),
		($1, $2, 'eth_getBalance',    'success',         80, false, 100, $3),
		($1, NULL,'eth_chainId',      'success',         90, true,  100, $3),
		($1, NULL,'eth_chainId',      'upstream_error', 100, false, 100, $3)
	`, statsTenantID, statsNetID, now)
	require.NoError(t, err)

	summary, err := db.GetStatsSummary(ctx, now.Add(-2*time.Minute), "1h")
	require.NoError(t, err)
	assert.Equal(t, "1h", summary.Range)

	assert.Equal(t, int64(10), summary.Totals.Requests, "total requests")
	assert.Equal(t, int64(7), summary.Totals.Success, "success count")
	assert.Equal(t, int64(3), summary.Totals.Errors, "error count")
	assert.Equal(t, int64(4), summary.Totals.CacheHits, "cache hits")
	assert.Equal(t, int64(6), summary.Totals.CacheMisses, "cache misses")

	assert.InDelta(t, 55.0, summary.Latency.AvgMS, 0.01, "average latency")
	assert.InDelta(t, 95.5, summary.Latency.P95MS, 0.01, "p95 latency")

	require.Len(t, summary.TopMethods, 4)
	assert.Equal(t, "eth_chainId", summary.TopMethods[0].Name)
	assert.Equal(t, int64(4), summary.TopMethods[0].Count)
	assert.Equal(t, "eth_blockNumber", summary.TopMethods[1].Name)
	assert.Equal(t, int64(3), summary.TopMethods[1].Count)
	assert.Equal(t, "eth_call", summary.TopMethods[2].Name)
	assert.Equal(t, int64(2), summary.TopMethods[2].Count)
	assert.Equal(t, "eth_getBalance", summary.TopMethods[3].Name)
	assert.Equal(t, int64(1), summary.TopMethods[3].Count)

	require.Len(t, summary.TopStatuses, 3)
	assert.Equal(t, "success", summary.TopStatuses[0].Name)
	assert.Equal(t, int64(7), summary.TopStatuses[0].Count)
	assert.Equal(t, "rpc_error", summary.TopStatuses[1].Name)
	assert.Equal(t, int64(2), summary.TopStatuses[1].Count)
	assert.Equal(t, "upstream_error", summary.TopStatuses[2].Name)
	assert.Equal(t, int64(1), summary.TopStatuses[2].Count)

	require.Len(t, summary.TopNetworks, 2)
	assert.Equal(t, "TestNet", summary.TopNetworks[0].Name)
	assert.Equal(t, int64(8), summary.TopNetworks[0].Count)
	assert.Equal(t, "unknown", summary.TopNetworks[1].Name)
	assert.Equal(t, int64(2), summary.TopNetworks[1].Count)

	require.Len(t, summary.Series, 1)
	assert.WithinDuration(t, now.Truncate(time.Minute), summary.Series[0].Time, time.Second)
	assert.Equal(t, int64(10), summary.Series[0].Requests)
	assert.Equal(t, int64(3), summary.Series[0].Errors)
	assert.Equal(t, int64(4), summary.Series[0].CacheHits)
}

func TestStatsSummary_RollupPath_AccuracyAndP95Approximation(t *testing.T) {
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
SELECT
	date_trunc('minute', created_at) AS bucket,
	COALESCE(network_id::text, '') AS network_id,
	method,
	status,
	cache_hit,
	COUNT(*) AS requests,
	COUNT(*) FILTER (WHERE status <> 'success') AS errors,
	COUNT(*) FILTER (WHERE cache_hit) AS cache_hits,
	COALESCE(AVG(latency_ms)::float8, 0) AS avg_latency_ms,
	COALESCE(percentile_cont(0.95) WITHIN GROUP (ORDER BY latency_ms)::float8, 0) AS p95_latency_ms
FROM request_logs
GROUP BY 1, 2, 3, 4, 5
WITH NO DATA
`)
	require.NoError(t, err)

	// Clean all request_logs and blockchain_configs
	_, err = db.pool.Exec(ctx, `TRUNCATE request_logs, blockchain_configs CASCADE`)
	require.NoError(t, err)

	bucket1 := time.Now().Add(-10 * time.Minute).Truncate(time.Minute)
	bucket2 := bucket1.Add(-1 * time.Minute)

	for i := 0; i < 9; i++ {
		_, err = db.pool.Exec(ctx,
			`INSERT INTO request_logs (tenant_id, method, status, latency_ms, cache_hit, created_at) VALUES ($1, 'eth_call', 'success', 10, false, $2)`,
			statsTenantID, bucket1,
		)
		require.NoError(t, err)
	}
	_, err = db.pool.Exec(ctx,
		`INSERT INTO request_logs (tenant_id, method, status, latency_ms, cache_hit, created_at) VALUES ($1, 'eth_call', 'success', 100, false, $2)`,
		statsTenantID, bucket1,
	)
	require.NoError(t, err)

	for i := 0; i < 9; i++ {
		_, err = db.pool.Exec(ctx,
			`INSERT INTO request_logs (tenant_id, method, status, latency_ms, cache_hit, created_at) VALUES ($1, 'eth_call', 'success', 10, false, $2)`,
			statsTenantID, bucket2,
		)
		require.NoError(t, err)
	}
	_, err = db.pool.Exec(ctx,
		`INSERT INTO request_logs (tenant_id, method, status, latency_ms, cache_hit, created_at) VALUES ($1, 'eth_call', 'success', 100, false, $2)`,
		statsTenantID, bucket2,
	)
	require.NoError(t, err)

	_, err = db.pool.Exec(ctx, `REFRESH MATERIALIZED VIEW request_logs_rollup_1m`)
	require.NoError(t, err)

	summary, err := db.GetStatsSummary(ctx, bucket2.Add(-5*time.Minute), "24h")
	require.NoError(t, err)
	assert.Equal(t, "24h", summary.Range)

	assert.Equal(t, int64(20), summary.Totals.Requests)
	assert.Equal(t, int64(20), summary.Totals.Success)
	assert.Equal(t, int64(0), summary.Totals.Errors)
	assert.Equal(t, int64(0), summary.Totals.CacheHits)
	assert.Equal(t, int64(20), summary.Totals.CacheMisses)

	assert.InDelta(t, 19.0, summary.Latency.AvgMS, 0.01, "rollup average is exact")
	assert.InDelta(t, 59.5, summary.Latency.P95MS, 0.01, "rollup p95 is MAX(per-minute p95), approximating true global p95 (which would be 100)")

	require.Len(t, summary.TopMethods, 1)
	assert.Equal(t, "eth_call", summary.TopMethods[0].Name)
	assert.Equal(t, int64(20), summary.TopMethods[0].Count)

	require.Len(t, summary.Series, 1)
	assert.WithinDuration(t, bucket2.Truncate(time.Hour), summary.Series[0].Time, time.Second)
	assert.Equal(t, int64(20), summary.Series[0].Requests)
	assert.Equal(t, int64(0), summary.Series[0].Errors)
	assert.Equal(t, int64(0), summary.Series[0].CacheHits)
}