package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime/debug"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/jeffTheItGuy/chainmesh/gateway/middleware"
	"github.com/jeffTheItGuy/chainmesh/shared/blockchain"
	"github.com/jeffTheItGuy/chainmesh/shared/logger"
	"github.com/jeffTheItGuy/chainmesh/shared/model"
	"github.com/jeffTheItGuy/chainmesh/shared/storage/postgres"
	"github.com/jeffTheItGuy/chainmesh/shared/storage/redis"
	"github.com/jeffTheItGuy/chainmesh/shared/telemetry"
	"github.com/jeffTheItGuy/chainmesh/shared/util"
)

type panicMockManager struct{}

func (m *panicMockManager) Get(networkID string) (*blockchain.Client, bool) {
	return nil, false
}

type mockNetworkManager struct {
	client *blockchain.Client
	ok     bool
}

func (m *mockNetworkManager) Get(networkID string) (*blockchain.Client, bool) {
	return m.client, m.ok
}

func setupProxyTestDeps(t *testing.T) (*postgres.DB, *redis.Client, *telemetry.Recorder, func()) {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	db, err := postgres.New(dsn)
	require.NoError(t, err)

	s := miniredis.RunT(t)
	cache := redis.New(s.Addr())

	rec := telemetry.New(db, logger.New(), 1000)
	rec.Start()

	cleanup := func() {
		rec.Stop()
		cache.Close()
		db.Close()
	}
	return db, cache, rec, cleanup
}

func TestProxy_MalformedRequestsDoNotPanic(t *testing.T) {
	db, cache, rec, cleanup := setupProxyTestDeps(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Seed a tenant so Auth middleware injects it into context.
	apiKey := util.GenerateAPIKey()
	_, err := db.CreateTenantWithKey(ctx, "ProxyPanicTest", "", 100, 10, 1000, "free", apiKey)
	require.NoError(t, err)

	p := New(&panicMockManager{}, db, cache, logger.New(), rec)
	handler := middleware.Auth(db)(p)

	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantInBody string
	}{
		{"empty_body", "", http.StatusBadRequest, "invalid json"},
		{"invalid_json", "not json", http.StatusBadRequest, "invalid json"},
		{"json_not_object", "[]", http.StatusBadRequest, "invalid json"},
		{"missing_jsonrpc", `{"method":"eth_chainId"}`, http.StatusBadRequest, "invalid json-rpc request"},
		{"missing_method", `{"jsonrpc":"2.0"}`, http.StatusBadRequest, "invalid json-rpc request"},
		{"method_not_string", `{"jsonrpc":"2.0","method":123}`, http.StatusBadRequest, "invalid json"},
		{"valid_no_network", `{"jsonrpc":"2.0","method":"eth_chainId","params":[],"id":1}`, http.StatusInternalServerError, "no blockchain network configured"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("panic recovered: %v\n%s", r, debug.Stack())
				}
			}()

			req := httptest.NewRequest("POST", "/v1/", bytes.NewBufferString(tt.body))
			req.Header.Set("Authorization", "Bearer "+apiKey)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code, "body: %s", rec.Body.String())
			assert.Contains(t, rec.Body.String(), tt.wantInBody)
		})
	}
}

func TestProxy_CacheHit(t *testing.T) {
	db, cache, rec, cleanup := setupProxyTestDeps(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Create a blockchain network and tenant linked to it.
	cfg, err := db.SaveBlockchainConfig(ctx, &model.BlockchainConfig{
		Name:         "CacheNet",
		RPCEndpoint1: "http://localhost:8545",
		ChainID:      "1337",
		Enabled:      true,
	})
	require.NoError(t, err)

	apiKey := util.GenerateAPIKey()
	_, err = db.CreateTenantWithKey(ctx, "CacheTenant", cfg.ID, 100, 10, 1000, "free", apiKey)
	require.NoError(t, err)

	// Pre-warm cache for eth_chainId with empty params.
	cacheKey := "rpc:" + cfg.ID + ":eth_chainId:[]"
	require.NoError(t, cache.Set(ctx, cacheKey, `{"jsonrpc":"2.0","result":"0x1","id":1}`, time.Hour))

	// Manager is never called on cache hit.
	p := New(&panicMockManager{}, db, cache, logger.New(), rec)
	handler := middleware.Auth(db)(p)

	req := httptest.NewRequest("POST", "/v1/", bytes.NewBufferString(`{"jsonrpc":"2.0","method":"eth_chainId","params":[],"id":1}`))
	req.Header.Set("Authorization", "Bearer "+apiKey)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "HIT", recorder.Header().Get("X-Cache"))
	assert.Contains(t, recorder.Body.String(), "0x1")
}

func TestProxy_CacheMissUpstreamSuccess(t *testing.T) {
	db, cache, rec, cleanup := setupProxyTestDeps(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Spin up a fake JSON-RPC node.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(blockchain.RPCResponse{
			JSONRPC: "2.0",
			Result:  json.RawMessage(`"0x539"`),
			ID:      1,
		})
	}))
	defer srv.Close()

	cfg, err := db.SaveBlockchainConfig(ctx, &model.BlockchainConfig{
		Name:         "SuccessNet",
		RPCEndpoint1: srv.URL,
		ChainID:      "1337",
		Enabled:      true,
	})
	require.NoError(t, err)

	apiKey := util.GenerateAPIKey()
	_, err = db.CreateTenantWithKey(ctx, "SuccessTenant", cfg.ID, 100, 10, 1000, "free", apiKey)
	require.NoError(t, err)

	bc := blockchain.New([]string{srv.URL})
	bc.SetNetworkID(cfg.ID)

	mgr := &mockNetworkManager{client: bc, ok: true}
	p := New(mgr, db, cache, logger.New(), rec)
	handler := middleware.Auth(db)(p)

	req := httptest.NewRequest("POST", "/v1/", bytes.NewBufferString(`{"jsonrpc":"2.0","method":"eth_chainId","params":[],"id":1}`))
	req.Header.Set("Authorization", "Bearer "+apiKey)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "MISS", recorder.Header().Get("X-Cache"))
	assert.Contains(t, recorder.Body.String(), "0x539")

	// Verify the response was cached for subsequent calls.
	cached, err := cache.Get(ctx, "rpc:"+cfg.ID+":eth_chainId:[]")
	require.NoError(t, err)
	assert.Contains(t, cached, "0x539")
}

func TestProxy_NetworkUnavailable(t *testing.T) {
	db, cache, rec, cleanup := setupProxyTestDeps(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cfg, err := db.SaveBlockchainConfig(ctx, &model.BlockchainConfig{
		Name:         "MissingNet",
		RPCEndpoint1: "http://localhost:8545",
		ChainID:      "1337",
		Enabled:      true,
	})
	require.NoError(t, err)

	apiKey := util.GenerateAPIKey()
	_, err = db.CreateTenantWithKey(ctx, "MissingTenant", cfg.ID, 100, 10, 1000, "free", apiKey)
	require.NoError(t, err)

	// Manager returns ok=false for this network.
	mgr := &mockNetworkManager{ok: false}
	p := New(mgr, db, cache, logger.New(), rec)
	handler := middleware.Auth(db)(p)

	req := httptest.NewRequest("POST", "/v1/", bytes.NewBufferString(`{"jsonrpc":"2.0","method":"eth_chainId","params":[],"id":1}`))
	req.Header.Set("Authorization", "Bearer "+apiKey)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "blockchain network unavailable")
}

func TestProxy_UpstreamError(t *testing.T) {
	db, cache, rec, cleanup := setupProxyTestDeps(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Use a port that is extremely unlikely to accept connections.
	bc := blockchain.New([]string{"http://127.0.0.1:1"})
	bc.SetNetworkID("dead-net")

	cfg, err := db.SaveBlockchainConfig(ctx, &model.BlockchainConfig{
		Name:         "DeadNet",
		RPCEndpoint1: "http://127.0.0.1:1",
		ChainID:      "1337",
		Enabled:      true,
	})
	require.NoError(t, err)

	apiKey := util.GenerateAPIKey()
	_, err = db.CreateTenantWithKey(ctx, "DeadTenant", cfg.ID, 100, 10, 1000, "free", apiKey)
	require.NoError(t, err)

	mgr := &mockNetworkManager{client: bc, ok: true}
	p := New(mgr, db, cache, logger.New(), rec)
	handler := middleware.Auth(db)(p)

	req := httptest.NewRequest("POST", "/v1/", bytes.NewBufferString(`{"jsonrpc":"2.0","method":"eth_chainId","params":[],"id":1}`))
	req.Header.Set("Authorization", "Bearer "+apiKey)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusBadGateway, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "upstream unavailable")
}

func TestProxy_RPCErrorResponse(t *testing.T) {
	db, cache, rec, cleanup := setupProxyTestDeps(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(blockchain.RPCResponse{
			JSONRPC: "2.0",
			Error:   &blockchain.RPCError{Code: -32000, Message: "execution reverted"},
			ID:      1,
		})
	}))
	defer srv.Close()

	cfg, err := db.SaveBlockchainConfig(ctx, &model.BlockchainConfig{
		Name:         "ErrorNet",
		RPCEndpoint1: srv.URL,
		ChainID:      "1337",
		Enabled:      true,
	})
	require.NoError(t, err)

	apiKey := util.GenerateAPIKey()
	_, err = db.CreateTenantWithKey(ctx, "ErrorTenant", cfg.ID, 100, 10, 1000, "free", apiKey)
	require.NoError(t, err)

	bc := blockchain.New([]string{srv.URL})
	bc.SetNetworkID(cfg.ID)

	mgr := &mockNetworkManager{client: bc, ok: true}
	p := New(mgr, db, cache, logger.New(), rec)
	handler := middleware.Auth(db)(p)

	req := httptest.NewRequest("POST", "/v1/", bytes.NewBufferString(`{"jsonrpc":"2.0","method":"eth_call","params":[],"id":1}`))
	req.Header.Set("Authorization", "Bearer "+apiKey)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "execution reverted")
}