package postgres

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/jeffTheItGuy/chainmesh/shared/model"
)

func TestBlockchainConfig_CRUD(t *testing.T) {
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
		)
	`)
	require.NoError(t, err)

	cfg := &model.BlockchainConfig{
		Name:         "TestNet",
		RPCEndpoint1: "http://localhost:8545",
		RPCEndpoint2: "http://localhost:8546",
		ChainID:      "1337",
		Enabled:      true,
	}

	saved, err := db.SaveBlockchainConfig(ctx, cfg)
	require.NoError(t, err)
	assert.NotEmpty(t, saved.ID)
	assert.Equal(t, "TestNet", saved.Name)

	fetched, err := db.GetBlockchainConfig(ctx, saved.ID)
	require.NoError(t, err)
	assert.Equal(t, saved.ID, fetched.ID)

	configs, err := db.ListBlockchainConfigs(ctx)
	require.NoError(t, err)
	assert.Len(t, configs, 1)

	defaultCfg, err := db.GetDefaultBlockchainConfig(ctx)
	require.NoError(t, err)
	assert.Equal(t, saved.ID, defaultCfg.ID)

	saved.Name = "UpdatedNet"
	err = db.UpdateBlockchainConfig(ctx, saved)
	require.NoError(t, err)

	updated, err := db.GetBlockchainConfig(ctx, saved.ID)
	require.NoError(t, err)
	assert.Equal(t, "UpdatedNet", updated.Name)

	err = db.DeleteBlockchainConfig(ctx, saved.ID)
	require.NoError(t, err)

	_, err = db.GetBlockchainConfig(ctx, saved.ID)
	assert.Error(t, err)
}

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
		CREATE TABLE IF NOT EXISTS tenants (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			name VARCHAR(255) NOT NULL,
			blockchain_network_id UUID REFERENCES blockchain_configs(id)
		);
		CREATE TABLE IF NOT EXISTS blocks (
			number BIGINT PRIMARY KEY,
			hash VARCHAR(66) NOT NULL,
			parent_hash VARCHAR(66) NOT NULL,
			timestamp TIMESTAMPTZ NOT NULL,
			tx_count INT NOT NULL DEFAULT 0,
			network_id UUID REFERENCES blockchain_configs(id),
			created_at TIMESTAMPTZ DEFAULT NOW()
		)
	`)
	require.NoError(t, err)

	cfg, err := db.SaveBlockchainConfig(ctx, &model.BlockchainConfig{
		Name:         "DeleteMe",
		RPCEndpoint1: "http://localhost:8545",
		Enabled:      true,
	})
	require.NoError(t, err)

	// Link a tenant and block
	_, err = db.pool.Exec(ctx,
		`INSERT INTO tenants (id, name, blockchain_network_id) VALUES (gen_random_uuid(), 'LinkTenant', $1)`,
		cfg.ID,
	)
	require.NoError(t, err)

	_, err = db.pool.Exec(ctx,
		`INSERT INTO blocks (number, hash, parent_hash, timestamp, network_id) VALUES (1, '0x1', '0x0', NOW(), $1)`,
		cfg.ID,
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
		`SELECT network_id FROM blocks WHERE number = 1`,
	).Scan(&blockNetID)
	require.NoError(t, err)
	assert.Nil(t, blockNetID)
}