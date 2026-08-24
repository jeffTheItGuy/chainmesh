package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/jeffTheItGuy/chainmesh/shared/requestid"
)

func TestRequestID_PropagatesExistingHeader(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := requestid.FromContext(r.Context())
		assert.Equal(t, "existing-id", id)
	})

	handler := RequestID(next)
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Request-ID", "existing-id")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	assert.Equal(t, "existing-id", rec.Header().Get("X-Request-ID"))
}

func TestRequestID_GeneratesNewID(t *testing.T) {
	var capturedID string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedID = requestid.FromContext(r.Context())
		assert.NotEmpty(t, capturedID)
	})

	handler := RequestID(next)
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	assert.Equal(t, capturedID, rec.Header().Get("X-Request-ID"))
}