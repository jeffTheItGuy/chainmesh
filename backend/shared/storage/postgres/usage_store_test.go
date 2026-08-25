// backend/shared/storage/postgres/usage_store_test.go
package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jeffTheItGuy/chainmesh/shared/model"
	// remove this line: "github.com/jeffTheItGuy/chainmesh/shared/util"
)

const usageTenantID = "880e8400-e29b-41d4-a716-446655440001"

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

	// Seed a tenant to satisfy FK
	_, err = db.pool.Exec(ctx,
		`INSERT INTO tenants (id, name) VALUES ($1, 'UsageTestTenant') ON CONFLICT (id) DO NOTHING`,
		usageTenantID,
	)
	require.NoError(t, err)

	// Clean up
	_, _ = db.pool.Exec(ctx, `DELETE FROM usage WHERE tenant_id = $1`, usageTenantID)

	period := time.Now().Truncate(time.Minute)
	u := &model.Usage{
		TenantID: usageTenantID,
		Method:   "eth_chainId",
		Count:    5,
		BytesIn:  100,
		Period:   period,
	}

	err = db.RecordUsage(ctx, u)
	require.NoError(t, err)

	// Upsert
	u.Count = 3
	u.BytesIn = 50
	err = db.RecordUsage(ctx, u)
	require.NoError(t, err)

	daily, err := db.GetDailyUsage(ctx, usageTenantID, period)
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

	usage, err := db.GetDailyUsage(ctx, "550e8400-e29b-41d4-a716-446655440099", time.Now())
	require.NoError(t, err)
	assert.Empty(t, usage)
}