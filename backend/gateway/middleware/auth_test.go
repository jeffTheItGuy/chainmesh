package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/jeffTheItGuy/chainmesh/shared/model"
)

type mockTenantResolver struct {
	tenant   *model.Tenant
	err      error
	callCount int
}

func (m *mockTenantResolver) GetTenantByAPIKey(ctx context.Context, key string) (*model.Tenant, error) {
	m.callCount++
	return m.tenant, m.err
}

func TestAuth_MissingHeader(t *testing.T) {
	handler := Auth(&mockTenantResolver{}, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next should not be called")
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "unauthorized")
}

func TestAuth_InvalidKey(t *testing.T) {
	resolver := &mockTenantResolver{err: errors.New("not found")}
	handler := Auth(resolver, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next should not be called")
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer invalid-key")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAuth_Success(t *testing.T) {
	tenant := &model.Tenant{ID: "tenant-1", Name: "Test"}
	resolver := &mockTenantResolver{tenant: tenant}
	var ctxTenant *model.Tenant

	handler := Auth(resolver, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctxTenant = TenantFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer valid-key")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, tenant, ctxTenant)
}

func TestTenantFromContext_Missing(t *testing.T) {
	assert.Nil(t, TenantFromContext(context.Background()))
}

// ---------------------------------------------------------------------------
// Cache-specific tests
// ---------------------------------------------------------------------------

func TestAuth_CacheHit_SkipsDB(t *testing.T) {
	tenant := &model.Tenant{ID: "tenant-cache", Name: "Cached"}
	resolver := &mockTenantResolver{tenant: tenant}
	cache := NewTenantCache(5 * time.Minute)

	var ctxTenant *model.Tenant
	handler := Auth(resolver, cache)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctxTenant = TenantFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	// First request: cache miss, hits the resolver
	req1 := httptest.NewRequest("GET", "/", nil)
	req1.Header.Set("Authorization", "Bearer cached-key")
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)

	assert.Equal(t, http.StatusOK, rec1.Code)
	assert.Equal(t, tenant, ctxTenant)
	assert.Equal(t, 1, resolver.callCount, "first request should hit the resolver")

	// Second request: cache hit, resolver should NOT be called again
	req2 := httptest.NewRequest("GET", "/", nil)
	req2.Header.Set("Authorization", "Bearer cached-key")
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	assert.Equal(t, http.StatusOK, rec2.Code)
	assert.Equal(t, tenant, ctxTenant)
	assert.Equal(t, 1, resolver.callCount, "second request should be served from cache")
}

func TestAuth_CacheExpiry(t *testing.T) {
	tenant := &model.Tenant{ID: "tenant-exp", Name: "Expiring"}
	resolver := &mockTenantResolver{tenant: tenant}
	cache := NewTenantCache(50 * time.Millisecond) // very short TTL for testing

	handler := Auth(resolver, cache)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer exp-key")

	// First request populates the cache
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req)
	assert.Equal(t, 1, resolver.callCount)

	// Wait for cache to expire
	time.Sleep(100 * time.Millisecond)

	// Second request should hit the resolver again
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req)
	assert.Equal(t, 2, resolver.callCount, "expired cache should hit the resolver again")
}

func TestAuth_CacheMiss_InvalidKey_NotCached(t *testing.T) {
	resolver := &mockTenantResolver{err: errors.New("not found")}
	cache := NewTenantCache(5 * time.Minute)

	handler := Auth(resolver, cache)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next should not be called")
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer bad-key")

	// First request: invalid key
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req)
	assert.Equal(t, http.StatusUnauthorized, rec1.Code)
	assert.Equal(t, 1, resolver.callCount)

	// Second request: invalid key should NOT be cached, resolver called again
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req)
	assert.Equal(t, http.StatusUnauthorized, rec2.Code)
	assert.Equal(t, 2, resolver.callCount, "failed auth should not be cached")
}