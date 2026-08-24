package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/jeffTheItGuy/chainmesh/shared/model"
	"github.com/jeffTheItGuy/chainmesh/shared/storage/redis"
)

func TestRateLimit_AllowsUnderQuota(t *testing.T) {
	s := miniredis.RunT(t)
	cache := redis.New(s.Addr())

	var called bool
	handler := RateLimit(cache)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	tenant := &model.Tenant{ID: "tenant-1", QuotaRPS: 10, QuotaRPM: 100, QuotaDaily: 10000}
	req := httptest.NewRequest("POST", "/", nil)
	req = req.WithContext(context.WithValue(req.Context(), tenantKey{}, tenant))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.True(t, called)
	assert.Equal(t, "100", rec.Header().Get("X-RateLimit-Limit-Minute"))
	assert.Equal(t, "99", rec.Header().Get("X-RateLimit-Remaining-Minute"))
}

func TestRateLimit_BlocksOverRPS(t *testing.T) {
	s := miniredis.RunT(t)
	cache := redis.New(s.Addr())

	handler := RateLimit(cache)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	tenant := &model.Tenant{ID: "tenant-2", QuotaRPS: 2, QuotaRPM: 100, QuotaDaily: 10000}

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("POST", "/", nil)
		req = req.WithContext(context.WithValue(req.Context(), tenantKey{}, tenant))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)
	}

	req := httptest.NewRequest("POST", "/", nil)
	req = req.WithContext(context.WithValue(req.Context(), tenantKey{}, tenant))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
	assert.Equal(t, "1", rec.Header().Get("Retry-After"))
}

func TestRateLimit_Race(t *testing.T) {
	s := miniredis.RunT(t)
	cache := redis.New(s.Addr())

	handler := RateLimit(cache)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	tenant := &model.Tenant{ID: "tenant-race", QuotaRPS: 0, QuotaRPM: 50, QuotaDaily: 0}

	var okCount, rejectCount atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest("POST", "/", nil)
			req = req.WithContext(context.WithValue(req.Context(), tenantKey{}, tenant))
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code == http.StatusOK {
				okCount.Add(1)
			} else {
				rejectCount.Add(1)
			}
		}()
	}
	wg.Wait()

	assert.Equal(t, int64(50), okCount.Load(), "exactly quota should pass")
	assert.Equal(t, int64(50), rejectCount.Load(), "remainder should be rejected")
}

func TestRateLimit_MissingTenant(t *testing.T) {
	s := miniredis.RunT(t)
	cache := redis.New(s.Addr())

	handler := RateLimit(cache)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not be called")
	}))

	req := httptest.NewRequest("POST", "/", nil) // no tenant in context
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}
