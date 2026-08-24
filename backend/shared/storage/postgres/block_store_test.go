package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/jeffTheItGuy/chainmesh/shared/model"
)

func TestBlock_StoreAndGet(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	_, err := db.pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS blockchain_configs (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			name TEXT NOT NULL,
			rpc_endpoint_1 TEXT NOT NULL,
			chain_id TEXT,
			enabled BOOLEAN NOT NULL DEFAULT true,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		CREATE TABLE IF NOT EXISTS blocks (
			number BIGINT PRIMARY KEY,
			hash VARCHAR(66) NOT NULL,
			parent_hash VARCHAR(66) NOT NULL,
			timestamp TIMESTAMPTZ NOT NULL,
			tx_count INT NOT NULL DEFAULT 0,
			raw_json JSONB,
			network_id UUID,
			created_at TIMESTAMPTZ DEFAULT NOW(),
			UNIQUE (number, network_id)
		)
	`)
	require.NoError(t, err)

	block := &model.Block{
		Number:     100,
		Hash:       "0xabc",
		ParentHash: "0xdef",
		Timestamp:  time.Now(),
		TxCount:    5,
		NetworkID:  "net-1",
		RawJSON:    []byte(`{"extra":"data"}`),
	}

	err = db.StoreBlock(ctx, block)
	require.NoError(t, err)

	latest, err := db.GetLatestBlock(ctx, "net-1")
	require.NoError(t, err)
	assert.Equal(t, int64(100), latest.Number)
	assert.Equal(t, "0xabc", latest.Hash)

	blocks, err := db.ListBlocks(ctx, 10)
	require.NoError(t, err)
	require.Len(t, blocks, 1)
	assert.Equal(t, int64(100), blocks[0].Number)
}

func TestBlock_StoreDuplicateIgnored(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	_, err := db.pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS blocks (
			number BIGINT PRIMARY KEY,
			hash VARCHAR(66) NOT NULL,
			parent_hash VARCHAR(66) NOT NULL,
			timestamp TIMESTAMPTZ NOT NULL,
			tx_count INT NOT NULL DEFAULT 0,
			raw_json JSONB,
			network_id UUID,
			created_at TIMESTAMPTZ DEFAULT NOW(),
			UNIQUE (number, network_id)
		)
	`)
	require.NoError(t, err)

	block := &model.Block{
		Number:     200,
		Hash:       "0xaaa",
		ParentHash: "0xbbb",
		Timestamp:  time.Now(),
		NetworkID:  "net-2",
	}

	require.NoError(t, db.StoreBlock(ctx, block))
	require.NoError(t, db.StoreBlock(ctx, block)) // ON CONFLICT DO NOTHING

	latest, err := db.GetLatestBlock(ctx, "net-2")
	require.NoError(t, err)
	assert.Equal(t, int64(200), latest.Number)
}