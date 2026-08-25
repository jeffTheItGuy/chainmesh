package telemetry

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jeffTheItGuy/chainmesh/shared/logger"
	"github.com/jeffTheItGuy/chainmesh/shared/model"
	"github.com/jeffTheItGuy/chainmesh/shared/storage/postgres"
)

func setupTelemetryTestDB(t *testing.T) *postgres.DB {
	t.Helper()

	var db *postgres.DB
	var err error

	if dsn := os.Getenv("TEST_DATABASE_URL"); dsn != "" {
		db, err = postgres.New(dsn)
		require.NoError(t, err)
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
			var innerErr error
			db, innerErr = postgres.New(dsn)
			return innerErr
		}))
	}

	t.Cleanup(db.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err = db.Pool().Exec(ctx, `
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

CREATE TABLE IF NOT EXISTS request_logs (
	id BIGSERIAL PRIMARY KEY,
	tenant_id UUID NOT NULL,
	network_id UUID,
	method TEXT NOT NULL,
	status TEXT NOT NULL,
	latency_ms INTEGER NOT NULL DEFAULT 0,
	cache_hit BOOLEAN NOT NULL DEFAULT false,
	bytes_in INTEGER NOT NULL DEFAULT 0,
	request_id TEXT,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS usage (
	tenant_id UUID NOT NULL,
	method VARCHAR(255) NOT NULL,
	count BIGINT NOT NULL DEFAULT 0,
	bytes_in BIGINT NOT NULL DEFAULT 0,
	period TIMESTAMPTZ NOT NULL,
	PRIMARY KEY (tenant_id, method, period)
);
`)
	require.NoError(t, err)

	tenantIDs := []string{
		"550e8400-e29b-41d4-a716-446655440001",
		"550e8400-e29b-41d4-a716-446655440002",
		"550e8400-e29b-41d4-a716-446655440003",
		"550e8400-e29b-41d4-a716-446655440004",
		"550e8400-e29b-41d4-a716-446655440005",
		"550e8400-e29b-41d4-a716-446655440006",
	}
	for _, id := range tenantIDs {
		_, err = db.Pool().Exec(ctx,
			`INSERT INTO tenants (id, name) VALUES ($1, $2) ON CONFLICT (id) DO NOTHING`,
			id, "TelemetryTestTenant",
		)
		require.NoError(t, err)
	}

	return db
}

func TestRecorder_StartStopNoGoroutineLeak(t *testing.T) {
	db := setupTelemetryTestDB(t)

	time.Sleep(100 * time.Millisecond)
	runtime.GC()
	baseline := runtime.NumGoroutine()

	recorder := New(db, logger.New(), 100)
	recorder.Start()
	time.Sleep(50 * time.Millisecond)
	recorder.Stop()

	time.Sleep(100 * time.Millisecond)
	runtime.GC()
	after := runtime.NumGoroutine()

	assert.LessOrEqual(t, after, baseline+2, "goroutine count should return to baseline after Stop")
}

func TestRecorder_StopIdempotency(t *testing.T) {
	db := setupTelemetryTestDB(t)

	recorder := New(db, logger.New(), 100)
	recorder.Start()
	time.Sleep(50 * time.Millisecond)
	recorder.Stop()
	recorder.Stop()
	recorder.Stop()

	recorder.RecordUsage(&model.Usage{
		TenantID: "550e8400-e29b-41d4-a716-446655440001",
		Method:   "eth_chainId",
		Count:    1,
		Period:   time.Now().Truncate(time.Minute),
	})
}

func TestRecorder_RecordUsage_LandsInDB(t *testing.T) {
	db := setupTelemetryTestDB(t)

	ctx := context.Background()
	_, err := db.Pool().Exec(ctx, `DELETE FROM usage WHERE tenant_id = $1`, "550e8400-e29b-41d4-a716-446655440002")
	require.NoError(t, err)

	recorder := New(db, logger.New(), 100)
	recorder.Start()
	defer recorder.Stop()

	period := time.Now().Truncate(time.Minute)
	tenantID := "550e8400-e29b-41d4-a716-446655440002"

	recorder.RecordUsage(&model.Usage{
		TenantID: tenantID,
		Method:   "eth_chainId",
		Count:    5,
		BytesIn:  100,
		Period:   period,
	})

	// Give the worker enough time to process
	time.Sleep(2 * time.Second)

	rows, err := db.Pool().Query(ctx,
		`SELECT method, count, bytes_in FROM usage WHERE tenant_id = $1 AND period = $2`,
		tenantID, period,
	)
	require.NoError(t, err)
	defer rows.Close()

	require.True(t, rows.Next())
	var method string
	var count, bytesIn int64
	require.NoError(t, rows.Scan(&method, &count, &bytesIn))
	assert.Equal(t, "eth_chainId", method)
	assert.Equal(t, int64(5), count)
	assert.Equal(t, int64(100), bytesIn)
}

func TestRecorder_RecordRequestLog_LandsInDB(t *testing.T) {
	db := setupTelemetryTestDB(t)

	recorder := New(db, logger.New(), 100)
	recorder.Start()
	defer recorder.Stop()

	tenantID := "550e8400-e29b-41d4-a716-446655440003"
	networkID := "550e8400-e29b-41d4-a716-446655440004"

	recorder.RecordRequestLog(&model.RequestLog{
		TenantID:  tenantID,
		NetworkID: networkID,
		Method:    "eth_call",
		Status:    "success",
		LatencyMS: 42,
		CacheHit:  true,
		BytesIn:   200,
		RequestID: "req-123",
	})

	time.Sleep(500 * time.Millisecond)

	ctx := context.Background()
	row := db.Pool().QueryRow(ctx,
		`SELECT method, status, latency_ms, cache_hit, request_id FROM request_logs WHERE tenant_id = $1`,
		tenantID,
	)

	var method, status, reqID string
	var latency int64
	var cacheHit bool
	require.NoError(t, row.Scan(&method, &status, &latency, &cacheHit, &reqID))
	assert.Equal(t, "eth_call", method)
	assert.Equal(t, "success", status)
	assert.Equal(t, int64(42), latency)
	assert.True(t, cacheHit)
	assert.Equal(t, "req-123", reqID)
}

func TestRecorder_QueueOverflow_DropsCleanly(t *testing.T) {
	db := setupTelemetryTestDB(t)

	recorder := New(db, logger.New(), 1)
	recorder.Start()
	defer recorder.Stop()

	tenantID := "550e8400-e29b-41d4-a716-446655440005"

	recorder.RecordUsage(&model.Usage{TenantID: tenantID, Method: "m1", Count: 1, Period: time.Now()})
	recorder.RecordUsage(&model.Usage{TenantID: tenantID, Method: "m2", Count: 1, Period: time.Now()})
}

func TestRecorder_DrainOnStop(t *testing.T) {
	db := setupTelemetryTestDB(t)

	recorder := New(db, logger.New(), 100)
	recorder.Start()

	ctx := context.Background()
	period := time.Now().Truncate(time.Minute)
	tenantID := "550e8400-e29b-41d4-a716-446655440006"

	for i := 0; i < 10; i++ {
		recorder.RecordUsage(&model.Usage{
			TenantID: tenantID,
			Method:   "eth_chainId",
			Count:    1,
			Period:   period,
		})
	}

	recorder.Stop()

	var count int64
	err := db.Pool().QueryRow(ctx,
		`SELECT COALESCE(SUM(count),0) FROM usage WHERE tenant_id = $1`,
		tenantID,
	).Scan(&count)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, count, int64(1), "at least one usage record should have drained to DB")
}