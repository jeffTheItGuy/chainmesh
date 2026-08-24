package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/jeffTheItGuy/chainmesh/shared/storage/postgres"
)

func setupAdminTestDB(t *testing.T) *postgres.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	db, err := postgres.New(dsn)
	require.NoError(t, err)
	t.Cleanup(db.Close)

	// Ensure audit_logs table exists when using an external test database.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err = db.Pool().Exec(ctx, `
		CREATE TABLE IF NOT EXISTS audit_logs (
			id BIGSERIAL PRIMARY KEY,
			actor TEXT NOT NULL,
			action TEXT NOT NULL,
			resource_type TEXT NOT NULL,
			resource_id TEXT,
			details JSONB,
			ip_address INET,
			user_agent TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	require.NoError(t, err)

	return db
}

func TestAdminAuth_Success(t *testing.T) {
	db := setupAdminTestDB(t)
	var called bool
	next := func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}

	handler := adminAuth("secret", db, next)
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Admin-Secret", "secret")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.True(t, called)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestAdminAuth_FailureAndAuditLog(t *testing.T) {
	db := setupAdminTestDB(t)
	next := func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next should not be called")
	}

	handler := adminAuth("secret", db, next)
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Admin-Secret", "wrong")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Contains(t, rec.Body.String(), "forbidden")

	// Audit log is async; allow a short window
	time.Sleep(150 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	logs, err := db.ListAuditLogs(ctx, 10, 0)
	require.NoError(t, err)

	var found bool
	for _, l := range logs {
		if l.Action == "ADMIN_AUTH_FAILURE" {
			found = true
			assert.Equal(t, "admin", l.Actor)
			assert.Equal(t, "admin", l.ResourceType)
			break
		}
	}
	assert.True(t, found, "expected ADMIN_AUTH_FAILURE audit log")
}

func TestAdminAuth_ConstantTimeRejection(t *testing.T) {
	db := setupAdminTestDB(t)
	next := func(w http.ResponseWriter, r *http.Request) {}

	handler := adminAuth("secret", db, next)

	for _, secret := range []string{"", "sec", "secret!", "SECRET"} {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("X-Admin-Secret", secret)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code, "secret %q should be rejected", secret)
	}
}
