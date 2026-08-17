package middleware

import (
	"net/http"

	"github.com/yourname/blockmesh/shared/requestid"
	"github.com/yourname/blockmesh/shared/util"
)

func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = util.GenerateRequestID()
		}

		ctx := requestid.NewContext(r.Context(), requestID)
		w.Header().Set("X-Request-ID", requestID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}