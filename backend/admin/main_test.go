package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/jeffTheItGuy/chainmesh/shared/logger"
	"github.com/jeffTheItGuy/chainmesh/shared/model"
	"github.com/jeffTheItGuy/chainmesh/shared/storage/postgres"
	"github.com/jeffTheItGuy/chainmesh/shared/util"
)

func setupAdminAPITestDB(t *testing.T) (*postgres.DB, string) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	db, err := postgres.New(dsn)
	require.NoError(t, err)
	t.Cleanup(db.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Ensure all required tables exist
	_, err = db.Pool().Exec(ctx, `
		CREATE TABLE IF NOT EXISTS audit_logs (
			id BIGSERIAL PRIMARY KEY,
			actor TEXT NOT NULL,
			action TEXT NOT NULL,
			resource_type TEXT NOT NULL,
			resource_id TEXT,
			details JSONB,
			ip_address INET,
			user_agent TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
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
			api_key VARCHAR(255) UNIQUE,
			quota_rpm INT NOT NULL DEFAULT 60,
			quota_rps INT NOT NULL DEFAULT 0,
			quota_daily INT NOT NULL DEFAULT 0,
			plan TEXT NOT NULL DEFAULT 'free',
			blockchain_network_id UUID,
			created_at TIMESTAMPTZ DEFAULT NOW()
		);
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
		CREATE TABLE IF NOT EXISTS blocks (
			number BIGINT PRIMARY KEY,
			hash VARCHAR(66) UNIQUE NOT NULL,
			parent_hash VARCHAR(66) NOT NULL,
			timestamp TIMESTAMPTZ NOT NULL,
			tx_count INT NOT NULL DEFAULT 0,
			raw_json JSONB,
			network_id UUID,
			created_at TIMESTAMPTZ DEFAULT NOW()
		);
		CREATE TABLE IF NOT EXISTS usage (
			tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
			method VARCHAR(255) NOT NULL,
			count BIGINT NOT NULL DEFAULT 0,
			bytes_in BIGINT NOT NULL DEFAULT 0,
			period TIMESTAMPTZ NOT NULL,
			PRIMARY KEY (tenant_id, method, period)
		);
		CREATE TABLE IF NOT EXISTS request_logs (
			id BIGSERIAL PRIMARY KEY,
			tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
			network_id UUID REFERENCES blockchain_configs(id) ON DELETE SET NULL,
			method TEXT NOT NULL,
			status TEXT NOT NULL,
			latency_ms INTEGER NOT NULL DEFAULT 0,
			cache_hit BOOLEAN NOT NULL DEFAULT false,
			bytes_in INTEGER NOT NULL DEFAULT 0,
			request_id TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		CREATE MATERIALIZED VIEW IF NOT EXISTS request_logs_rollup_1m AS
		SELECT date_trunc('minute', created_at) AS bucket, COALESCE(network_id::text, '') AS network_id,
		       method, status, cache_hit, COUNT(*) AS requests,
		       COUNT(*) FILTER (WHERE status <> 'success') AS errors,
		       COUNT(*) FILTER (WHERE cache_hit) AS cache_hits,
		       COALESCE(AVG(latency_ms)::float8, 0) AS avg_latency_ms,
		       COALESCE(percentile_cont(0.95) WITHIN GROUP (ORDER BY latency_ms)::float8, 0) AS p95_latency_ms
		FROM request_logs GROUP BY 1, 2, 3, 4, 5 WITH NO DATA;
	`)
	require.NoError(t, err)

	// Clean leftovers
	_, _ = db.Pool().Exec(ctx, `DELETE FROM tenants WHERE name LIKE 'AdminTest%';`)
	_, _ = db.Pool().Exec(ctx, `DELETE FROM blockchain_configs WHERE name LIKE 'AdminTest%';`)

	return db, "admin-secret"
}

func TestAdmin_Health(t *testing.T) {
	db, secret := setupAdminAPITestDB(t)
	mux := newAdminMux(secret, db, logger.New())

	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "ok")
}

func TestAdmin_StatsSummary(t *testing.T) {
	db, secret := setupAdminAPITestDB(t)
	mux := newAdminMux(secret, db, logger.New())

	tests := []struct {
		name       string
		query      string
		wantStatus int
		wantInBody string
	}{
		{"default_range", "", http.StatusOK, "range"},
		{"valid_15m", "?range=15m", http.StatusOK, "range"},
		{"valid_24h", "?range=24h", http.StatusOK, "range"},
		{"invalid_range", "?range=7d", http.StatusBadRequest, "range must be one of"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/stats/summary"+tt.query, nil)
			req.Header.Set("X-Admin-Secret", secret)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code, rec.Body.String())
			assert.Contains(t, rec.Body.String(), tt.wantInBody)
		})
	}
}

func TestAdmin_Tenants_CRUD(t *testing.T) {
	db, secret := setupAdminAPITestDB(t)
	mux := newAdminMux(secret, db, logger.New())
	ctx := context.Background()

	// Seed a default blockchain config so tenant creation can auto-link
	cfg, err := db.SaveBlockchainConfig(ctx, &model.BlockchainConfig{
		Name:         "AdminTest_Net",
		RPCEndpoint1: "https://ethereum-rpc.publicnode.com",
		ChainID:      "1",
		Enabled:      true,
	})
	require.NoError(t, err)

	t.Run("create_missing_name", func(t *testing.T) {
		body := `{"quota_rpm":100}`
		req := httptest.NewRequest("POST", "/tenants", bytes.NewBufferString(body))
		req.Header.Set("X-Admin-Secret", secret)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), "name is required")
	})

	t.Run("create_success", func(t *testing.T) {
		body := `{"name":"AdminTest_Create","quota_rpm":200,"quota_rps":20,"quota_daily":50000,"plan":"pro","blockchain_network_id":"` + cfg.ID + `"}`
		req := httptest.NewRequest("POST", "/tenants", bytes.NewBufferString(body))
		req.Header.Set("X-Admin-Secret", secret)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusCreated, rec.Code)
		assert.Contains(t, rec.Body.String(), "api_key")
	})

	t.Run("list", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/tenants", nil)
		req.Header.Set("X-Admin-Secret", secret)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	var tenantID string
	t.Run("get", func(t *testing.T) {
		// First create one
		body := `{"name":"AdminTest_Get","blockchain_network_id":"` + cfg.ID + `"}`
		req := httptest.NewRequest("POST", "/tenants", bytes.NewBufferString(body))
		req.Header.Set("X-Admin-Secret", secret)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		require.Equal(t, http.StatusCreated, rec.Code)

		var result map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &result))
		tenantID = result["id"].(string)

		// Now get it
		req = httptest.NewRequest("GET", "/tenants/"+tenantID, nil)
		req.Header.Set("X-Admin-Secret", secret)
		rec = httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), "AdminTest_Get")
	})

	t.Run("put", func(t *testing.T) {
		body := `{"name":"AdminTest_Updated"}`
		req := httptest.NewRequest("PUT", "/tenants/"+tenantID, bytes.NewBufferString(body))
		req.Header.Set("X-Admin-Secret", secret)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), "updated")
	})

	t.Run("rotate_key", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/tenants/"+tenantID+"/rotate-key", nil)
		req.Header.Set("X-Admin-Secret", secret)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), "api_key")
	})

	t.Run("usage", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/tenants/"+tenantID+"/usage", nil)
		req.Header.Set("X-Admin-Secret", secret)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("delete", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/tenants/"+tenantID, nil)
		req.Header.Set("X-Admin-Secret", secret)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), "deleted")
	})
}

func TestAdmin_Blocks_Public(t *testing.T) {
	db, _ := setupAdminAPITestDB(t)
	mux := newAdminMux("unused", db, logger.New())

	req := httptest.NewRequest("GET", "/blocks", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestAdmin_Blockchain_TestEndpoint(t *testing.T) {
	db, secret := setupAdminAPITestDB(t)
	mux := newAdminMux(secret, db, logger.New())

	t.Run("missing_endpoint", func(t *testing.T) {
		body := `{}`
		req := httptest.NewRequest("POST", "/blockchain/test", bytes.NewBufferString(body))
		req.Header.Set("X-Admin-Secret", secret)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("invalid_endpoint", func(t *testing.T) {
		body := `{"rpc_endpoint_1":"http://127.0.0.1:8545"}`
		req := httptest.NewRequest("POST", "/blockchain/test", bytes.NewBufferString(body))
		req.Header.Set("X-Admin-Secret", secret)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), "restricted ip")
	})
}

func TestAdmin_Blockchain_CRUD(t *testing.T) {
	db, secret := setupAdminAPITestDB(t)
	mux := newAdminMux(secret, db, logger.New())

	t.Run("create", func(t *testing.T) {
		body := `{"name":"AdminTest_BC","rpc_endpoint_1":"https://ethereum-rpc.publicnode.com","chain_id":"1","enabled":true}`
		req := httptest.NewRequest("POST", "/blockchain", bytes.NewBufferString(body))
		req.Header.Set("X-Admin-Secret", secret)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusCreated, rec.Code)
	})

	t.Run("list", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/blockchain", nil)
		req.Header.Set("X-Admin-Secret", secret)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	var bcID string
	t.Run("get_by_id", func(t *testing.T) {
		// Create first
		body := `{"name":"AdminTest_BC_Get","rpc_endpoint_1":"https://ethereum-rpc.publicnode.com","chain_id":"1","enabled":true}`
		req := httptest.NewRequest("POST", "/blockchain", bytes.NewBufferString(body))
		req.Header.Set("X-Admin-Secret", secret)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		require.Equal(t, http.StatusCreated, rec.Code)

		var result model.BlockchainConfig
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &result))
		bcID = result.ID

		req = httptest.NewRequest("GET", "/blockchain/"+bcID, nil)
		req.Header.Set("X-Admin-Secret", secret)
		rec = httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("put", func(t *testing.T) {
		body := `{"name":"AdminTest_BC_Updated","rpc_endpoint_1":"https://ethereum-rpc.publicnode.com","chain_id":"1","enabled":true}`
		req := httptest.NewRequest("PUT", "/blockchain/"+bcID, bytes.NewBufferString(body))
		req.Header.Set("X-Admin-Secret", secret)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("delete", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/blockchain/"+bcID, nil)
		req.Header.Set("X-Admin-Secret", secret)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), "deleted")
	})
}

func TestAdmin_AuditLogs(t *testing.T) {
	db, secret := setupAdminAPITestDB(t)
	mux := newAdminMux(secret, db, logger.New())

	req := httptest.NewRequest("GET", "/audit-logs?limit=10&offset=0", nil)
	req.Header.Set("X-Admin-Secret", secret)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestAdmin_MethodNotAllowed(t *testing.T) {
	db, secret := setupAdminAPITestDB(t)
	mux := newAdminMux(secret, db, logger.New())

	req := httptest.NewRequest("PATCH", "/tenants", nil)
	req.Header.Set("X-Admin-Secret", secret)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}

func TestAdmin_GatewayHealthNodes_Proxy(t *testing.T) {
	db, _ := setupAdminAPITestDB(t)
	mux := newAdminMux("secret", db, logger.New())

	// No gateway running — expect 502
	req := httptest.NewRequest("GET", "/gateway/health/nodes", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadGateway, rec.Code)
	assert.Contains(t, rec.Body.String(), "gateway unreachable")
}

func TestAdmin_AuthRequired(t *testing.T) {
	db, _ := setupAdminAPITestDB(t)
	mux := newAdminMux("secret", db, logger.New())

	req := httptest.NewRequest("GET", "/tenants", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Contains(t, rec.Body.String(), "forbidden")
}

func TestAdmin_InvalidAuth(t *testing.T) {
	db, _ := setupAdminAPITestDB(t)
	mux := newAdminMux("secret", db, logger.New())

	req := httptest.NewRequest("GET", "/tenants", nil)
	req.Header.Set("X-Admin-Secret", "wrong")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}