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

func setupManagerTestDB(t *testing.T) *postgres.DB {
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
`)
	require.NoError(t, err)

	// FIX: Delete ALL configs to ensure test isolation.
	// Other test packages (proxy, postgres) leave configs behind in the shared DB.
	_, err = db.Pool().Exec(ctx, `DELETE FROM blockchain_configs`)
	require.NoError(t, err)

	return db
}

func rpcServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(blockchain.RPCResponse{
			JSONRPC: "2.0",
			Result:  json.RawMessage(`"0x1"`),
			ID:      1,
		})
	}))
}

func TestManager_Start_EmptyConfigs(t *testing.T) {
	db := setupManagerTestDB(t)
	m := NewManager(db, logger.New())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := m.Start(ctx)
	require.NoError(t, err)
	m.Stop()
}

func TestManager_Reload_AddConfig(t *testing.T) {
	db := setupManagerTestDB(t)
	srv := rpcServer()
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cfg, err := db.SaveBlockchainConfig(ctx, &model.BlockchainConfig{
		Name:         "ManagerTest_Add",
		RPCEndpoint1: srv.URL,
		ChainID:      "1",
		Enabled:      true,
	})
	require.NoError(t, err)

	m := NewManager(db, logger.New())
	require.NoError(t, m.reload(ctx))
	defer m.Stop()

	assert.Len(t, m.clients, 1)
	assert.Contains(t, m.sigs, cfg.ID)

	client, ok := m.Get(cfg.ID)
	assert.True(t, ok)
	assert.NotNil(t, client)
}

func TestManager_Reload_UpdateConfig(t *testing.T) {
	db := setupManagerTestDB(t)
	srv1 := rpcServer()
	defer srv1.Close()
	srv2 := rpcServer()
	defer srv2.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cfg, err := db.SaveBlockchainConfig(ctx, &model.BlockchainConfig{
		Name:         "ManagerTest_Update",
		RPCEndpoint1: srv1.URL,
		ChainID:      "1",
		Enabled:      true,
	})
	require.NoError(t, err)

	m := NewManager(db, logger.New())
	require.NoError(t, m.reload(ctx))
	defer m.Stop()

	oldSig := m.sigs[cfg.ID]
	oldClient := m.clients[cfg.ID]
	require.NotNil(t, oldClient)

	_, err = db.Pool().Exec(ctx,
		`UPDATE blockchain_configs SET rpc_endpoint_1 = $1 WHERE id = $2`,
		srv2.URL, cfg.ID,
	)
	require.NoError(t, err)

	require.NoError(t, m.reload(ctx))

	newSig := m.sigs[cfg.ID]
	newClient := m.clients[cfg.ID]

	assert.NotEqual(t, oldSig, newSig, "signature should change when endpoint changes")
	assert.NotSame(t, oldClient, newClient, "client should be replaced")
}

func TestManager_Reload_RemoveConfig(t *testing.T) {
	db := setupManagerTestDB(t)
	srv := rpcServer()
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cfg, err := db.SaveBlockchainConfig(ctx, &model.BlockchainConfig{
		Name:         "ManagerTest_Remove",
		RPCEndpoint1: srv.URL,
		ChainID:      "1",
		Enabled:      true,
	})
	require.NoError(t, err)

	m := NewManager(db, logger.New())
	require.NoError(t, m.reload(ctx))
	assert.Contains(t, m.clients, cfg.ID)

	_, err = db.Pool().Exec(ctx,
		`UPDATE blockchain_configs SET enabled = false WHERE id = $1`,
		cfg.ID,
	)
	require.NoError(t, err)

	require.NoError(t, m.reload(ctx))

	assert.NotContains(t, m.clients, cfg.ID)
	assert.NotContains(t, m.sigs, cfg.ID)

	m.Stop()
}

func TestManager_Get_NotFound(t *testing.T) {
	db := setupManagerTestDB(t)
	m := NewManager(db, logger.New())

	client, ok := m.Get("non-existent")
	assert.False(t, ok)
	assert.Nil(t, client)
}

func TestManager_Health(t *testing.T) {
	db := setupManagerTestDB(t)
	srv := rpcServer()
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cfg, err := db.SaveBlockchainConfig(ctx, &model.BlockchainConfig{
		Name:         "ManagerTest_Health",
		RPCEndpoint1: srv.URL,
		ChainID:      "1",
		Enabled:      true,
	})
	require.NoError(t, err)

	m := NewManager(db, logger.New())
	require.NoError(t, m.reload(ctx))
	defer m.Stop()

	time.Sleep(200 * time.Millisecond)

	health := m.Health()
	require.Len(t, health, 1)
	assert.Equal(t, cfg.ID, health[0].NetworkID)
	require.Len(t, health[0].Nodes, 1)
	assert.Equal(t, srv.URL, health[0].Nodes[0].URL)
}

func TestManager_Stop_ClearsClients(t *testing.T) {
	db := setupManagerTestDB(t)
	srv := rpcServer()
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := db.SaveBlockchainConfig(ctx, &model.BlockchainConfig{
		Name:         "ManagerTest_Stop",
		RPCEndpoint1: srv.URL,
		ChainID:      "1",
		Enabled:      true,
	})
	require.NoError(t, err)

	m := NewManager(db, logger.New())
	require.NoError(t, m.reload(ctx))
	assert.NotEmpty(t, m.clients)

	m.Stop()

	assert.Empty(t, m.clients)
	assert.Empty(t, m.sigs)
}

func TestConfigSignature(t *testing.T) {
	a := configSignature(model.BlockchainConfig{
		RPCEndpoint1: "http://a",
		RPCEndpoint2: "http://b",
		Enabled:      true,
	})
	b := configSignature(model.BlockchainConfig{
		RPCEndpoint1: "http://a",
		RPCEndpoint2: "http://b",
		Enabled:      true,
	})
	c := configSignature(model.BlockchainConfig{
		RPCEndpoint1: "http://a",
		RPCEndpoint2: "http://c",
		Enabled:      true,
	})

	assert.Equal(t, a, b)
	assert.NotEqual(t, a, c)
}