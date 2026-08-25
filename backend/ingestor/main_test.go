// backend/ingestor/main_test.go
package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jeffTheItGuy/chainmesh/shared/blockchain"
	"github.com/jeffTheItGuy/chainmesh/shared/logger"
	"github.com/jeffTheItGuy/chainmesh/shared/model"
	"github.com/jeffTheItGuy/chainmesh/shared/storage/postgres"
)

// Use valid UUIDs instead of arbitrary strings
const ingestTestNetID1 = "aa0e8400-e29b-41d4-a716-446655440001"
const ingestTestNetID2 = "aa0e8400-e29b-41d4-a716-446655440002"
const ingestTestNetID3 = "aa0e8400-e29b-41d4-a716-446655440003"

func setupIngestorTestDB(t *testing.T) *postgres.DB {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	db, err := postgres.New(dsn)
	require.NoError(t, err)
	t.Cleanup(db.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err = db.Pool().Exec(ctx, `
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
	hash VARCHAR(66) UNIQUE NOT NULL,
	parent_hash VARCHAR(66) NOT NULL,
	timestamp TIMESTAMPTZ NOT NULL,
	tx_count INT NOT NULL DEFAULT 0,
	raw_json JSONB,
	network_id UUID,
	created_at TIMESTAMPTZ DEFAULT NOW(),
	UNIQUE(number, network_id)
);

ALTER TABLE blocks ADD COLUMN IF NOT EXISTS network_id UUID;
`)
	require.NoError(t, err)

	return db
}

func TestFetchAndStore_Success(t *testing.T) {
	db := setupIngestorTestDB(t)
	ctx := context.Background()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0",
			"result": map[string]any{
				"number":       "0x10",
				"hash":         "0xabc123",
				"parentHash":   "0xdef456",
				"timestamp":    "0x6155",
				"transactions": []string{"0x1", "0x2"},
			},
			"id": 1,
		})
	}))
	defer srv.Close()

	cfg := &model.BlockchainConfig{
		ID:           ingestTestNetID1, // now a UUID
		Name:         "TestNet",
		RPCEndpoint1: srv.URL,
		Enabled:      true,
	}

	err := fetchAndStore(ctx, cfg.ID, newTestClient(cfg), db, logger.New())
	require.NoError(t, err)

	block, err := db.GetLatestBlock(ctx, cfg.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(16), block.Number)
	assert.Equal(t, "0xabc123", block.Hash)
	assert.Equal(t, 2, block.TxCount)
}

func TestFetchAndStore_RPCError(t *testing.T) {
	db := setupIngestorTestDB(t)
	ctx := context.Background()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0",
			"error": map[string]any{
				"code":    -32000,
				"message": "execution reverted",
			},
			"id": 1,
		})
	}))
	defer srv.Close()

	cfg := &model.BlockchainConfig{
		ID:           ingestTestNetID2,
		Name:         "TestNet",
		RPCEndpoint1: srv.URL,
		Enabled:      true,
	}

	err := fetchAndStore(ctx, cfg.ID, newTestClient(cfg), db, logger.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rpc error")
}

func TestFetchAndStore_MalformedBlock(t *testing.T) {
	db := setupIngestorTestDB(t)
	ctx := context.Background()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0",
			"result": map[string]any{
				"number":    "not-a-hex",
				"hash":      "0xabc",
				"timestamp": "0x0",
			},
			"id": 1,
		})
	}))
	defer srv.Close()

	cfg := &model.BlockchainConfig{
		ID:           ingestTestNetID3,
		Name:         "TestNet",
		RPCEndpoint1: srv.URL,
		Enabled:      true,
	}

	err := fetchAndStore(ctx, cfg.ID, newTestClient(cfg), db, logger.New())
	require.Error(t, err) // strconv.ParseInt should fail
}

func newTestClient(cfg *model.BlockchainConfig) *blockchain.Client {
	endpoints := []string{cfg.RPCEndpoint1}
	if cfg.RPCEndpoint2 != "" {
		endpoints = append(endpoints, cfg.RPCEndpoint2)
	}
	bc := blockchain.New(endpoints)
	bc.SetNetworkID(cfg.ID)
	return bc
}