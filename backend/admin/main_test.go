// backend/admin/main_test.go
package main

import (
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
	"github.com/jeffTheItGuy/chainmesh/shared/storage/postgres"
)

const testAdminSecret = "test-secret-key"

func setupAdminTestDB(t *testing.T) *postgres.DB {
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
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
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

	return db
}

func newTestMux(db *postgres.DB) *http.ServeMux {
	return newAdminMux(testAdminSecret, db, logger.New())
}

func doRequest(mux *http.ServeMux, method, path, body string, withAuth bool) *httptest.ResponseRecorder {
	var reqBody *strings.Reader
	if body != "" {
		reqBody = strings.NewReader(body)
	} else {
		reqBody = strings.NewReader("")
	}
	req := httptest.NewRequest(method, path, reqBody)
	req.Header.Set("Content-Type", "application/json")
	if withAuth {
		req.Header.Set("X-Admin-Secret", testAdminSecret)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestHealth_NoAuthRequired(t *testing.T) {
	db := setupAdminTestDB(t)
	mux := newTestMux(db)

	rec := doRequest(mux, http.MethodGet, "/health", "", false)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]string
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.Equal(t, "ok", resp["status"])
}

func TestAuth_ForbiddenWithoutSecret(t *testing.T) {
	db := setupAdminTestDB(t)
	mux := newTestMux(db)

	rec := doRequest(mux, http.MethodGet, "/tenants", "", false)
	assert.Equal(t, http.StatusForbidden, rec.Code)

	var resp map[string]string
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.Equal(t, "forbidden", resp["error"])
}

func TestAuth_ForbiddenWithWrongSecret(t *testing.T) {
	db := setupAdminTestDB(t)
	mux := newTestMux(db)

	req := httptest.NewRequest(http.MethodGet, "/tenants", nil)
	req.Header.Set("X-Admin-Secret", "wrong-secret")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestTenants_CreateAndGet(t *testing.T) {
	db := setupAdminTestDB(t)
	mux := newTestMux(db)

	body := `{"name":"TestTenant","quota_rpm":100,"quota_rps":10,"quota_daily":1000,"plan":"pro"}`
	rec := doRequest(mux, http.MethodPost, "/tenants", body, true)
	assert.Equal(t, http.StatusCreated, rec.Code)

	var created map[string]any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&created))
	assert.Equal(t, "TestTenant", created["name"])
	assert.NotEmpty(t, created["id"])
	assert.NotEmpty(t, created["api_key"])

	tenantID := created["id"].(string)

	rec = doRequest(mux, http.MethodGet, "/tenants/"+tenantID, "", true)
	assert.Equal(t, http.StatusOK, rec.Code)

	var fetched map[string]any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&fetched))
	assert.Equal(t, "TestTenant", fetched["name"])
}

func TestTenants_CreateRequiresName(t *testing.T) {
	db := setupAdminTestDB(t)
	mux := newTestMux(db)

	body := `{"quota_rpm":100}`
	rec := doRequest(mux, http.MethodPost, "/tenants", body, true)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	var resp map[string]string
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.Contains(t, resp["error"], "name is required")
}

func TestTenants_RotateKey(t *testing.T) {
	db := setupAdminTestDB(t)
	mux := newTestMux(db)

	body := `{"name":"RotateTenant","quota_rpm":60}`
	rec := doRequest(mux, http.MethodPost, "/tenants", body, true)
	require.Equal(t, http.StatusCreated, rec.Code)

	var created map[string]any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&created))
	tenantID := created["id"].(string)

	rec = doRequest(mux, http.MethodPost, "/tenants/"+tenantID+"/rotate-key", "", true)
	assert.Equal(t, http.StatusOK, rec.Code)

	var rotated map[string]string
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&rotated))
	assert.NotEmpty(t, rotated["api_key"])
	assert.True(t, strings.HasPrefix(rotated["api_key"], "bm_live_"))
}

func TestTenants_Delete(t *testing.T) {
	db := setupAdminTestDB(t)
	mux := newTestMux(db)

	body := `{"name":"DeleteMe"}`
	rec := doRequest(mux, http.MethodPost, "/tenants", body, true)
	require.Equal(t, http.StatusCreated, rec.Code)

	var created map[string]any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&created))
	tenantID := created["id"].(string)

	rec = doRequest(mux, http.MethodDelete, "/tenants/"+tenantID, "", true)
	assert.Equal(t, http.StatusOK, rec.Code)

	rec = doRequest(mux, http.MethodGet, "/tenants/"+tenantID, "", true)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestBlockchain_CreateAndGet(t *testing.T) {
	db := setupAdminTestDB(t)
	mux := newTestMux(db)

	body := `{"name":"TestNet","rpc_endpoint_1":"http://1.1.1.1:8545","chain_id":"1","enabled":true}`
	rec := doRequest(mux, http.MethodPost, "/blockchain", body, true)
	assert.Equal(t, http.StatusCreated, rec.Code)

	var created map[string]any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&created))
	assert.Equal(t, "TestNet", created["name"])
	assert.NotEmpty(t, created["id"])

	networkID := created["id"].(string)

	rec = doRequest(mux, http.MethodGet, "/blockchain/"+networkID, "", true)
	assert.Equal(t, http.StatusOK, rec.Code)

	var fetched map[string]any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&fetched))
	assert.Equal(t, "TestNet", fetched["name"])
}

func TestBlockchain_CreateRequiresFields(t *testing.T) {
	db := setupAdminTestDB(t)
	mux := newTestMux(db)

	body := `{"name":"Incomplete"}`
	rec := doRequest(mux, http.MethodPost, "/blockchain", body, true)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestBlockchain_Delete(t *testing.T) {
	db := setupAdminTestDB(t)
	mux := newTestMux(db)

	body := `{"name":"DeleteNet","rpc_endpoint_1":"http://1.1.1.1:8545","enabled":true}`
	rec := doRequest(mux, http.MethodPost, "/blockchain", body, true)
	require.Equal(t, http.StatusCreated, rec.Code)

	var created map[string]any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&created))
	networkID := created["id"].(string)

	rec = doRequest(mux, http.MethodDelete, "/blockchain/"+networkID, "", true)
	assert.Equal(t, http.StatusOK, rec.Code)

	rec = doRequest(mux, http.MethodGet, "/blockchain/"+networkID, "", true)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}