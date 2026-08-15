package middleware

import (
	"net/http"

	"github.com/yourname/blockmesh/shared/storage/redis"
)

func RateLimit(cache *redis.Client) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenant := TenantFromContext(r.Context())
			if tenant == nil {
				next.ServeHTTP(w, r)
				return
			}

			allowed, err := cache.CheckRateLimit(r.Context(), tenant.ID, tenant.QuotaRPM)
			if err != nil || !allowed {
				w.Header().Set("Content-Type", "application/json")
				http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
