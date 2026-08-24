package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/jeffTheItGuy/chainmesh/shared/model"
)

func TestUsage_UpsertAndGet(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	_, err := db.pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS usage (
			tenant_id UUID NOT NULL,
			method VARCHAR(255) NOT NULL,
			count BIGINT NOT NULL DEFAULT 0,
			bytes_in BIGINT NOT NULL DEFAULT 0,
			period TIMESTAMPTZ NOT NULL,
			PRIMARY KEY (tenant_id, method, period)
		)
	`)
	require.NoError(t, err)

	period := time.Now().Truncate(time.Minute)
	u := &model.Usage{
		TenantID: "tenant-usage",
		Method:   "eth_chainId",
		Count:    5,
		BytesIn:  100,
		Period:   period,
	}

	err = db.RecordUsage(ctx, u)
	require.NoError(t, err)

	// Upsert should add
	u.Count = 3
	u.BytesIn = 50
	err = db.RecordUsage(ctx, u)
	require.NoError(t, err)

	daily, err := db.GetDailyUsage(ctx, "tenant-usage", period)
	require.NoError(t, err)
	require.Len(t, daily, 1)
	assert.Equal(t, int64(8), daily[0].Count)
	assert.Equal(t, int64(150), daily[0].BytesIn)
}

func TestUsage_GetDailyEmpty(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	_, err := db.pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS usage (
			tenant_id UUID NOT NULL,
			method VARCHAR(255) NOT NULL,
			count BIGINT NOT NULL DEFAULT 0,
			bytes_in BIGINT NOT NULL DEFAULT 0,
			period TIMESTAMPTZ NOT NULL,
			PRIMARY KEY (tenant_id, method, period)
		)
	`)
	require.NoError(t, err)

	usage, err := db.GetDailyUsage(ctx, "nonexistent", time.Now())
	require.NoError(t, err)
	assert.Empty(t, usage)
}