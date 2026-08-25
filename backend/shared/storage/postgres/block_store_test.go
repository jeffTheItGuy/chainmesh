package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jeffTheItGuy/chainmesh/shared/model"
)

const blockTestNetID1 = "bb0e8400-e29b-41d4-a716-446655440001"
const blockTestNetID2 = "bb0e8400-e29b-41d4-a716-446655440002"

func setupBlockTestSchema(t *testing.T, db *DB) {
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

CREATE TABLE IF NOT EXISTS blocks (
	number BIGINT PRIMARY KEY,
	hash VARCHAR(66) NOT NULL,
	parent_hash VARCHAR(66) NOT NULL,
	timestamp TIMESTAMPTZ NOT NULL,
	tx_count INT NOT NULL DEFAULT 0,
	raw_json JSONB,
	network_id UUID,
	created_at TIMESTAMPTZ DEFAULT NOW()
);

ALTER TABLE blocks ADD COLUMN IF NOT EXISTS network_id UUID;

-- FIX: Ensure the composite unique constraint exists for ON CONFLICT (number, network_id)
ALTER TABLE blocks DROP CONSTRAINT IF EXISTS blocks_number_network_key;
ALTER TABLE blocks ADD CONSTRAINT blocks_number_network_key UNIQUE (number, network_id);
`)
	require.NoError(t, err)
}

func TestBlock_StoreAndGet(t *testing.T) {
	db := setupTestDB(t)
	setupBlockTestSchema(t, db)
	ctx := context.Background()

	// FIX: Clean up to avoid conflicts from other tests
	_, _ = db.pool.Exec(ctx, `DELETE FROM blocks WHERE network_id = $1`, blockTestNetID1)

	block := &model.Block{
		Number:     100,
		Hash:       "0xabc",
		ParentHash: "0xdef",
		Timestamp:  time.Now(),
		TxCount:    5,
		NetworkID:  blockTestNetID1,
		RawJSON:    []byte(`{"extra":"data"}`),
	}

	err := db.StoreBlock(ctx, block)
	require.NoError(t, err)

	latest, err := db.GetLatestBlock(ctx, blockTestNetID1)
	require.NoError(t, err)
	assert.Equal(t, int64(100), latest.Number)
	assert.Equal(t, "0xabc", latest.Hash)

	blocks, err := db.ListBlocks(ctx, 10)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(blocks), 1)
}

func TestBlock_StoreDuplicateIgnored(t *testing.T) {
	db := setupTestDB(t)
	setupBlockTestSchema(t, db)
	ctx := context.Background()

	// FIX: Clean up to avoid conflicts
	_, _ = db.pool.Exec(ctx, `DELETE FROM blocks WHERE network_id = $1`, blockTestNetID2)

	block := &model.Block{
		Number:     200,
		Hash:       "0xaaa",
		ParentHash: "0xbbb",
		Timestamp:  time.Now(),
		NetworkID:  blockTestNetID2,
	}

	require.NoError(t, db.StoreBlock(ctx, block))
	require.NoError(t, db.StoreBlock(ctx, block)) // ON CONFLICT DO NOTHING

	latest, err := db.GetLatestBlock(ctx, blockTestNetID2)
	require.NoError(t, err)
	assert.Equal(t, int64(200), latest.Number)
}