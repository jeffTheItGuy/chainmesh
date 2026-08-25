package postgres

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jeffTheItGuy/chainmesh/shared/util"
)

func setupTestDB(t *testing.T) *DB {
	t.Helper()

	var db *DB

	if dsn := os.Getenv("TEST_DATABASE_URL"); dsn != "" {
		var err error
		db, err = New(dsn)
		require.NoError(t, err)
		t.Cleanup(db.Close)
	} else {
		pool, err := dockertest.NewPool("")
		require.NoError(t, err)

		resource, err := pool.RunWithOptions(&dockertest.RunOptions{
			Repository: "postgres",
			Tag:        "16-alpine",
			Env: []string{
				"POSTGRES_USER=test",
				"POSTGRES_PASSWORD=test",
				"POSTGRES_DB=test",
			},
		}, func(config *docker.HostConfig) {
			config.AutoRemove = true
			config.RestartPolicy = docker.RestartPolicy{Name: "no"}
		})
		require.NoError(t, err)

		t.Cleanup(func() { _ = pool.Purge(resource) })

		dsn := fmt.Sprintf("postgres://test:test@%s/test?sslmode=disable", resource.GetHostPort("5432/tcp"))

		require.NoError(t, pool.Retry(func() error {
			var err error
			db, err = New(dsn)
			return err
		}))
		t.Cleanup(db.Close)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := db.pool.Exec(ctx, `
		CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
		CREATE TABLE IF NOT EXISTS tenants (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			name VARCHAR(255) NOT NULL,
			api_key VARCHAR(255) UNIQUE,
			quota_rpm INT NOT NULL DEFAULT 60,
			quota_rps INT NOT NULL DEFAULT 0,
			quota_daily INT NOT NULL DEFAULT 0,
			plan TEXT NOT NULL DEFAULT 'free',
			blockchain_network_id UUID,
			created_at TIMESTAMPTZ DEFAULT NOW()
		);
		CREATE TABLE IF NOT EXISTS api_keys (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
			name TEXT NOT NULL DEFAULT 'default',
			key_hash TEXT NOT NULL UNIQUE,
			key_prefix TEXT NOT NULL,
			revoked_at TIMESTAMPTZ,
			last_used_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
	`)
	require.NoError(t, err)

	return db
}

func TestCreateTenantWithKey(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	key := util.GenerateAPIKey()

	tenant, err := db.CreateTenantWithKey(ctx, "Acme", "", 100, 10, 1000, "pro", key)
	require.NoError(t, err)
	assert.Equal(t, "Acme", tenant.Name)
	assert.Equal(t, 100, tenant.QuotaRPM)
	assert.Equal(t, "pro", tenant.Plan)

	fetched, err := db.GetTenantByID(ctx, tenant.ID)
	require.NoError(t, err)
	assert.Equal(t, tenant.ID, fetched.ID)
}

func TestRotateAPIKey(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Generate a unique name to avoid collisions
	uniqueName := fmt.Sprintf("RotateMe_%d", time.Now().UnixNano())

	// Clean up any leftover from previous runs (though the name is unique)
	_, err := db.pool.Exec(ctx, `DELETE FROM tenants WHERE name = $1`, uniqueName)
	require.NoError(t, err)

	oldKey := util.GenerateAPIKey()
	newKey := util.GenerateAPIKey()

	tenant, err := db.CreateTenantWithKey(ctx, uniqueName, "", 60, 5, 500, "free", oldKey)
	require.NoError(t, err)

	err = db.RotateAPIKey(ctx, tenant.ID, newKey)
	require.NoError(t, err)

	_, err = db.GetTenantByAPIKey(ctx, oldKey)
	assert.Error(t, err, "old key should be revoked")

	fetched, err := db.GetTenantByAPIKey(ctx, newKey)
	require.NoError(t, err)
	assert.Equal(t, tenant.ID, fetched.ID)

	// Clean up after test
	_, _ = db.pool.Exec(ctx, `DELETE FROM tenants WHERE id = $1`, tenant.ID)
}

func TestGetTenantByAPIKey_WrongKeyRejected(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	correctKey := util.GenerateAPIKey()

	_, err := db.CreateTenantWithKey(ctx, "Secure", "", 60, 5, 500, "free", correctKey)
	require.NoError(t, err)

	_, err = db.GetTenantByAPIKey(ctx, util.GenerateAPIKey())
	assert.Error(t, err)
}