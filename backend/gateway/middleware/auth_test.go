package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/jeffTheItGuy/chainmesh/shared/model"
)

type mockTenantResolver struct {
	tenant *model.Tenant
	err    error
}

func (m *mockTenantResolver) GetTenantByAPIKey(ctx context.Context, key string) (*model.Tenant, error) {
	return m.tenant, m.err
}

func TestAuth_MissingHeader(t *testing.T) {
	handler := Auth(&mockTenantResolver{})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	handler := Auth(resolver)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	handler := Auth(resolver)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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