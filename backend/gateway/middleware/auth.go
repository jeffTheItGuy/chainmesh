package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/yourname/blockmesh/shared/model"
	"github.com/yourname/blockmesh/shared/storage/postgres"
)

type tenantKey struct{}

func TenantFromContext(ctx context.Context) *model.Tenant {
	t, _ := ctx.Value(tenantKey{}).(*model.Tenant)
	return t
}

func Auth(db *postgres.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auth := r.Header.Get("Authorization")
			if !strings.HasPrefix(auth, "Bearer ") {
				w.Header().Set("Content-Type", "application/json")
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}

			key := strings.TrimPrefix(auth, "Bearer ")
			tenant, err := db.GetTenantByAPIKey(r.Context(), key)
			if err != nil {
				w.Header().Set("Content-Type", "application/json")
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), tenantKey{}, tenant)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}