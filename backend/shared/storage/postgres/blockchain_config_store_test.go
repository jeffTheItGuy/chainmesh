package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jeffTheItGuy/chainmesh/shared/model"
)

func TestBlockchainConfig_DeleteUnlinksRefs(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	_, err := db.pool.Exec(ctx, `
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

CREATE TABLE IF NOT EXISTS tenants (
	id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	name VARCHAR(255) NOT NULL,
	blockchain_network_id UUID REFERENCES blockchain_configs(id)
);

ALTER TABLE tenants ADD COLUMN IF NOT EXISTS blockchain_network_id UUID;

CREATE TABLE IF NOT EXISTS blocks (
	number BIGINT PRIMARY KEY,
	hash VARCHAR(66) NOT NULL,
	parent_hash VARCHAR(66) NOT NULL,
	timestamp TIMESTAMPTZ NOT NULL,
	tx_count INT NOT NULL DEFAULT 0,
	network_id UUID REFERENCES blockchain_configs(id),
	created_at TIMESTAMPTZ DEFAULT NOW()
);

ALTER TABLE blocks ADD COLUMN IF NOT EXISTS network_id UUID;
`)
	require.NoError(t, err)

	cfg, err := db.SaveBlockchainConfig(ctx, &model.BlockchainConfig{
		Name:         "DeleteMe",
		RPCEndpoint1: "http://localhost:8545",
		Enabled:      true,
	})
	require.NoError(t, err)

	_, err = db.pool.Exec(ctx,
		`INSERT INTO tenants (id, name, blockchain_network_id) VALUES (gen_random_uuid(), 'LinkTenant', $1)`,
		cfg.ID,
	)
	require.NoError(t, err)

	// FIX: Use a unique block number (based on current time) to avoid PK conflicts
	uniqueBlockNum := time.Now().UnixNano() % 1000000000
	_, err = db.pool.Exec(ctx,
		`INSERT INTO blocks (number, hash, parent_hash, timestamp, network_id) VALUES ($1, $2, '0x0', NOW(), $3)`,
		uniqueBlockNum, "0xunlink", cfg.ID,
	)
	require.NoError(t, err)

	err = db.DeleteBlockchainConfig(ctx, cfg.ID)
	require.NoError(t, err)

	var tenantNetID *string
	err = db.pool.QueryRow(ctx,
		`SELECT blockchain_network_id FROM tenants WHERE name = 'LinkTenant'`,
	).Scan(&tenantNetID)
	require.NoError(t, err)
	assert.Nil(t, tenantNetID)

	var blockNetID *string
	err = db.pool.QueryRow(ctx,
		`SELECT network_id FROM blocks WHERE number = $1`,
		uniqueBlockNum,
	).Scan(&blockNetID)
	require.NoError(t, err)
	assert.Nil(t, blockNetID)

	// Cleanup
	_, _ = db.pool.Exec(ctx, `DELETE FROM tenants WHERE name = 'LinkTenant'`)
	_, _ = db.pool.Exec(ctx, `DELETE FROM blocks WHERE number = $1`, uniqueBlockNum)
}