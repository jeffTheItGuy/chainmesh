package proxy

import (
	"bytes"
	"context"
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
	"github.com/jeffTheItGuy/chainmesh/shared/storage/postgres"
	"github.com/jeffTheItGuy/chainmesh/shared/storage/redis"
	"github.com/jeffTheItGuy/chainmesh/shared/telemetry"
	"github.com/jeffTheItGuy/chainmesh/shared/util"
)

type panicMockManager struct{}

func (m *panicMockManager) Get(networkID string) (*blockchain.Client, bool) {
	return nil, false
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
