package middleware

import (
	"net/http"
	"strconv"

	"github.com/jeffTheItGuy/chainmesh/shared/metrics"
	"github.com/jeffTheItGuy/chainmesh/shared/storage/redis"
)

func RateLimit(cache *redis.Client) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenant := TenantFromContext(r.Context())
			if tenant == nil {
				w.Header().Set("Content-Type", "application/json")
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}

			allowed, status, err := cache.CheckRateLimits(
				r.Context(),
				tenant.ID,
				tenant.QuotaRPS,
				tenant.QuotaRPM,
				tenant.QuotaDaily,
			)
			if err != nil {
				metrics.RateLimitErrorsTotal.Inc()
				w.Header().Set("Content-Type", "application/json")
				http.Error(w, `{"error":"rate limiter unavailable"}`, http.StatusServiceUnavailable)
				return
			}

			if status.LimitMinute > 0 {
				w.Header().Set("X-RateLimit-Limit-Minute", strconv.Itoa(status.LimitMinute))
				w.Header().Set("X-RateLimit-Remaining-Minute", strconv.Itoa(status.RemainingMinute))
				w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(status.ResetUnix, 10))
			}

			if status.RetryAfterSeconds > 0 {
				w.Header().Set("Retry-After", strconv.Itoa(status.RetryAfterSeconds))
			}

			if !allowed {
				metrics.RateLimitedTotal.WithLabelValues(status.RejectedLimit).Inc()
				w.Header().Set("Content-Type", "application/json")
				http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
