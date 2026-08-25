package postgres

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jeffTheItGuy/chainmesh/shared/model"
)

const reqLogTenantID1 = "550e8400-e29b-41d4-a716-446655440001"
const reqLogTenantID2 = "550e8400-e29b-41d4-a716-446655440002"
const reqLogNetID1 = "660e8400-e29b-41d4-a716-446655440001"

func TestRequestLog_RecordAndRetrieve(t *testing.T) {
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
)
`)
	require.NoError(t, err)

	// FIX: Clean up request_logs for this tenant to ensure test isolation
	_, _ = db.pool.Exec(ctx, `DELETE FROM request_logs WHERE tenant_id = $1`, reqLogTenantID1)

	log := &model.RequestLog{
		TenantID:  reqLogTenantID1,
		NetworkID: reqLogNetID1,
		Method:    "eth_chainId",
		Status:    "success",
		LatencyMS: 42,
		CacheHit:  true,
		BytesIn:   100,
		RequestID: "req-abc",
	}

	err = db.RecordRequestLog(ctx, log)
	require.NoError(t, err)

	var count int
	err = db.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM request_logs WHERE tenant_id = $1 AND request_id = $2`,
		reqLogTenantID1, "req-abc",
	).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestRequestLog_EmptyNetworkID(t *testing.T) {
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
)
`)
	require.NoError(t, err)

	// FIX: Clean up
	_, _ = db.pool.Exec(ctx, `DELETE FROM request_logs WHERE tenant_id = $1`, reqLogTenantID2)

	log := &model.RequestLog{
		TenantID:  reqLogTenantID2,
		Method:    "eth_call",
		Status:    "error",
		LatencyMS: 100,
		BytesIn:   50,
	}

	err = db.RecordRequestLog(ctx, log)
	require.NoError(t, err)

	var nid *string
	err = db.pool.QueryRow(ctx,
		`SELECT network_id FROM request_logs WHERE tenant_id = $1`,
		reqLogTenantID2,
	).Scan(&nid)
	require.NoError(t, err)
	assert.Nil(t, nid)
}