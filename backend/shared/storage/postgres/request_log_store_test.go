package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/jeffTheItGuy/chainmesh/shared/model"
)

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

	log := &model.RequestLog{
		TenantID:  "tenant-1",
		NetworkID: "net-1",
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
		"tenant-1", "req-abc",
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

	log := &model.RequestLog{
		TenantID: "tenant-2",
		Method:   "eth_call",
		Status:   "error",
		LatencyMS: 100,
		BytesIn:  50,
	}

	err = db.RecordRequestLog(ctx, log)
	require.NoError(t, err)

	var nid *string
	err = db.pool.QueryRow(ctx,
		`SELECT network_id FROM request_logs WHERE tenant_id = $1`,
		"tenant-2",
	).Scan(&nid)
	require.NoError(t, err)
	assert.Nil(t, nid)
}