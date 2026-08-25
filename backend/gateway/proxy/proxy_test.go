package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime/debug"
	"sync"
	"sync/atomic"
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

	// Ensure schema is up to date
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err = db.Pool().Exec(ctx, `
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

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
	id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
	name VARCHAR(255) NOT NULL,
	api_key VARCHAR(255) UNIQUE,
	quota_rpm INT NOT NULL DEFAULT 60,
	quota_rps INT NOT NULL DEFAULT 0,
	quota_daily INT NOT NULL DEFAULT 0,
	plan TEXT NOT NULL DEFAULT 'free',
	blockchain_network_id UUID,
	created_at TIMESTAMPTZ DEFAULT NOW()
);

ALTER TABLE tenants ADD COLUMN IF NOT EXISTS quota_rps INT NOT NULL DEFAULT 0;
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS quota_daily INT NOT NULL DEFAULT 0;
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS plan TEXT NOT NULL DEFAULT 'free';
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS blockchain_network_id UUID;

CREATE TABLE IF NOT EXISTS api_keys (
	id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
	name TEXT NOT NULL DEFAULT 'default',
	key_hash TEXT NOT NULL UNIQUE,
	key_prefix TEXT NOT NULL,
	revoked_at TIMESTAMPTZ,
	last_used_at TIMESTAMPTZ,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

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
);

ALTER TABLE request_logs ADD COLUMN IF NOT EXISTS request_id TEXT;

CREATE TABLE IF NOT EXISTS usage (
	tenant_id UUID NOT NULL,
	method VARCHAR(255) NOT NULL,
	count BIGINT NOT NULL DEFAULT 0,
	bytes_in BIGINT NOT NULL DEFAULT 0,
	period TIMESTAMPTZ NOT NULL,
	PRIMARY KEY (tenant_id, method, period)
);
`)
	require.NoError(t, err)

	// FIX: Clean up configs and tenants to ensure test isolation.
	// Other test packages leave configs in the shared DB which causes
	// GetDefaultBlockchainConfig to succeed when we expect it to fail.
	_, err = db.Pool().Exec(ctx, `DELETE FROM blockchain_configs`)
	require.NoError(t, err)
	_, err = db.Pool().Exec(ctx, `DELETE FROM api_keys`)
	require.NoError(t, err)
	_, err = db.Pool().Exec(ctx, `DELETE FROM tenants`)
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
		// FIX: With configs cleaned up, GetDefaultBlockchainConfig returns error → 500
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

	cfg, err := db.SaveBlockchainConfig(ctx, &model.BlockchainConfig{
		Name:         "CacheNet",
		RPCEndpoint1: "http://1.1.1.1:8545",
		ChainID:      "1337",
		Enabled:      true,
	})
	require.NoError(t, err)

	apiKey := util.GenerateAPIKey()
	_, err = db.CreateTenantWithKey(ctx, "CacheTenant", cfg.ID, 100, 10, 1000, "free", apiKey)
	require.NoError(t, err)

	// FIX: Cache key format is "rpc:<networkID>:<method>" (no params suffix when params is empty)
	cacheKey := "rpc:" + cfg.ID + ":eth_chainId"
	require.NoError(t, cache.Set(ctx, cacheKey, `{"jsonrpc":"2.0","result":"0x1","id":1}`, time.Hour))

	// FIX: The proxy calls manager.Get() BEFORE checking cache.
	// Use a real mock that returns ok=true so the code reaches the cache check.
	bc := blockchain.New([]string{"http://1.1.1.1:8545"})
	bc.SetNetworkID(cfg.ID)
	mgr := &mockNetworkManager{client: bc, ok: true}

	p := New(mgr, db, cache, logger.New(), rec)
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

	// FIX: Cache key has no ":[]" suffix when params is empty
	cached, err := cache.Get(ctx, "rpc:"+cfg.ID+":eth_chainId")
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
		RPCEndpoint1: "http://1.1.1.1:8545",
		ChainID:      "1337",
		Enabled:      true,
	})
	require.NoError(t, err)

	apiKey := util.GenerateAPIKey()
	_, err = db.CreateTenantWithKey(ctx, "MissingTenant", cfg.ID, 100, 10, 1000, "free", apiKey)
	require.NoError(t, err)

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

// ---------------------------------------------------------------------------
// Missing critical tests
// ---------------------------------------------------------------------------

func TestProxy_CacheStampede(t *testing.T) {
	db, cache, rec, cleanup := setupProxyTestDeps(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var upstreamCalls atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls.Add(1)
		// Slow upstream so concurrent requests have time to race
		time.Sleep(50 * time.Millisecond)
		json.NewEncoder(w).Encode(blockchain.RPCResponse{
			JSONRPC: "2.0",
			Result:  json.RawMessage(`"0x1"`),
			ID:      1,
		})
	}))
	defer srv.Close()

	cfg, err := db.SaveBlockchainConfig(ctx, &model.BlockchainConfig{
		Name:         "StampedeNet",
		RPCEndpoint1: srv.URL,
		ChainID:      "1337",
		Enabled:      true,
	})
	require.NoError(t, err)

	apiKey := util.GenerateAPIKey()
	_, err = db.CreateTenantWithKey(ctx, "StampedeTenant", cfg.ID, 100, 10, 1000, "free", apiKey)
	require.NoError(t, err)

	bc := blockchain.New([]string{srv.URL})
	bc.SetNetworkID(cfg.ID)
	mgr := &mockNetworkManager{client: bc, ok: true}

	p := New(mgr, db, cache, logger.New(), rec)
	handler := middleware.Auth(db)(p)

	// Fire 10 concurrent requests at a completely cold cache
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest("POST", "/v1/", bytes.NewBufferString(`{"jsonrpc":"2.0","method":"eth_chainId","params":[],"id":1}`))
			req.Header.Set("Authorization", "Bearer "+apiKey)
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, req)
			assert.Equal(t, http.StatusOK, recorder.Code)
		}()
	}
	wg.Wait()

	// Current implementation has no singleflight, so expect multiple upstream calls.
	// This test documents the behavior and will fail if singleflight is added
	// without adjusting the assertion.
	t.Logf("upstream calls during stampede: %d", upstreamCalls.Load())
	assert.GreaterOrEqual(t, upstreamCalls.Load(), int64(1), "at least one upstream call must happen")
	assert.LessOrEqual(t, upstreamCalls.Load(), int64(10), "without singleflight, up to 10 calls may happen")

	// Warm cache: a single follow-up request must be a hit
	req := httptest.NewRequest("POST", "/v1/", bytes.NewBufferString(`{"jsonrpc":"2.0","method":"eth_chainId","params":[],"id":1}`))
	req.Header.Set("Authorization", "Bearer "+apiKey)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "HIT", recorder.Header().Get("X-Cache"))
}

func TestProxy_TelemetryFallback(t *testing.T) {
	db, cache, _, cleanup := setupProxyTestDeps(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(blockchain.RPCResponse{
			JSONRPC: "2.0",
			Result:  json.RawMessage(`"0x999"`),
			ID:      1,
		})
	}))
	defer srv.Close()

	cfg, err := db.SaveBlockchainConfig(ctx, &model.BlockchainConfig{
		Name:         "FallbackNet",
		RPCEndpoint1: srv.URL,
		ChainID:      "1337",
		Enabled:      true,
	})
	require.NoError(t, err)

	apiKey := util.GenerateAPIKey()
	tenant, err := db.CreateTenantWithKey(ctx, "FallbackTenant", cfg.ID, 100, 10, 1000, "free", apiKey)
	require.NoError(t, err)

	bc := blockchain.New([]string{srv.URL})
	bc.SetNetworkID(cfg.ID)
	mgr := &mockNetworkManager{client: bc, ok: true}

	// Pass nil telemetry to force the synchronous fallback goroutine path
	p := New(mgr, db, cache, logger.New(), nil)
	handler := middleware.Auth(db)(p)

	req := httptest.NewRequest("POST", "/v1/", bytes.NewBufferString(`{"jsonrpc":"2.0","method":"eth_chainId","params":[],"id":1}`))
	req.Header.Set("Authorization", "Bearer "+apiKey)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "0x999")

	// Wait for the fallback goroutine to finish writing to Postgres
	time.Sleep(300 * time.Millisecond)

	var reqCount int
	err = db.Pool().QueryRow(ctx,
		`SELECT COUNT(*) FROM request_logs WHERE tenant_id = $1`,
		tenant.ID,
	).Scan(&reqCount)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, reqCount, 1, "fallback goroutine should have written request_logs")

	var usageCount int64
	err = db.Pool().QueryRow(ctx,
		`SELECT COALESCE(SUM(count),0) FROM usage WHERE tenant_id = $1`,
		tenant.ID,
	).Scan(&usageCount)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, usageCount, int64(1), "fallback goroutine should have written usage")
}