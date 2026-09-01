package middleware

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/jeffTheItGuy/chainmesh/shared/model"
)

type tenantKey struct{}

type tenantResolver interface {
	GetTenantByAPIKey(ctx context.Context, key string) (*model.Tenant, error)
}

// --- ADD THIS IN-MEMORY CACHE ---
type cachedTenant struct {
	tenant    *model.Tenant
	expiresAt time.Time
}

type TenantCache struct {
	mu    sync.RWMutex
	items map[string]cachedTenant
	ttl   time.Duration
}

func NewTenantCache(ttl time.Duration) *TenantCache {
	return &TenantCache{
		items: make(map[string]cachedTenant),
		ttl:   ttl,
	}
}

func (c *TenantCache) Get(key string) (*model.Tenant, bool) {
	c.mu.RLock()
	item, ok := c.items[key]
	c.mu.RUnlock()
	if !ok || time.Now().After(item.expiresAt) {
		return nil, false
	}
	return item.tenant, true
}

func (c *TenantCache) Set(key string, tenant *model.Tenant) {
	c.mu.Lock()
	c.items[key] = cachedTenant{
		tenant:    tenant,
		expiresAt: time.Now().Add(c.ttl),
	}
	c.mu.Unlock()
}
// --------------------------------

func TenantFromContext(ctx context.Context) *model.Tenant {
	t, _ := ctx.Value(tenantKey{}).(*model.Tenant)
	return t
}

// UPDATE SIGNATURE: Add cache parameter
func Auth(db tenantResolver, cache *TenantCache) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auth := r.Header.Get("Authorization")
			if !strings.HasPrefix(auth, "Bearer ") {
				w.Header().Set("Content-Type", "application/json")
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			key := strings.TrimPrefix(auth, "Bearer ")
			
			// 1. Check Cache First (0 CPU cost, ~0.001ms latency)
			if cache != nil {
				if tenant, ok := cache.Get(key); ok {
					ctx := context.WithValue(r.Context(), tenantKey{}, tenant)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
			}

			// 2. Cache Miss: Hit Postgres + Bcrypt (High CPU cost)
			tenant, err := db.GetTenantByAPIKey(r.Context(), key)
			if err != nil {
				w.Header().Set("Content-Type", "application/json")
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			
			// 3. Store in Cache for next time
			if cache != nil {
				cache.Set(key, tenant)
			}

			ctx := context.WithValue(r.Context(), tenantKey{}, tenant)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}