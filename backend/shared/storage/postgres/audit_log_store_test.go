package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/jeffTheItGuy/chainmesh/shared/model"
)

func TestAuditLog_RecordAndList(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	_, err := db.pool.Exec(ctx, `
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

	log := &model.AuditLog{
		Actor:        "admin",
		Action:       "TEST_ACTION",
		ResourceType: "test",
		ResourceID:   "res-1",
		Details:      []byte(`{"foo":"bar"}`),
		IPAddress:    "192.168.1.1",
		UserAgent:    "test-agent",
		CreatedAt:    time.Now(),
	}

	err = db.RecordAuditLog(ctx, log)
	require.NoError(t, err)

	logs, err := db.ListAuditLogs(ctx, 10, 0)
	require.NoError(t, err)
	require.Len(t, logs, 1)
	assert.Equal(t, "TEST_ACTION", logs[0].Action)
	assert.Equal(t, "admin", logs[0].Actor)
	assert.Equal(t, "res-1", logs[0].ResourceID)
	assert.Equal(t, "192.168.1.1", logs[0].IPAddress)
}

func TestAuditLog_ListPagination(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	_, err := db.pool.Exec(ctx, `
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

	for i := 0; i < 5; i++ {
		require.NoError(t, db.RecordAuditLog(ctx, &model.AuditLog{
			Actor:        "admin",
			Action:       "BULK_ACTION",
			ResourceType: "test",
			CreatedAt:    time.Now(),
		}))
	}

	logs, err := db.ListAuditLogs(ctx, 2, 0)
	require.NoError(t, err)
	assert.Len(t, logs, 2)

	logs, err = db.ListAuditLogs(ctx, 100, 0)
	require.NoError(t, err)
	assert.Len(t, logs, 5)
}