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

	if dsn := os.Getenv("TEST_DATABASE_URL"); dsn != "" {
		db, err := postgres.New(dsn)
		require.NoError(t, err)
		t.Cleanup(db.Close)
		return db
	}

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

	var db *postgres.DB
	require.NoError(t, pool.Retry(func() error {
		var innerErr error
		db, innerErr = postgres.New(dsn)
		return innerErr
	}))
	t.Cleanup(db.Close)

	// Ensure schema exists
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, _ = db.Pool().Exec(ctx, `
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
		TenantID: "t1",
		Method:   "eth_chainId",
		Count:    1,
		Period:   time.Now().Truncate(time.Minute),
	})
}

func TestRecorder_RecordUsage_LandsInDB(t *testing.T) {
	db := setupTelemetryTestDB(t)
	recorder := New(db, logger.New(), 100)
	recorder.Start()
	defer recorder.Stop()

	ctx := context.Background()
	period := time.Now().Truncate(time.Minute)

	recorder.RecordUsage(&model.Usage{
		TenantID: "tenant-usage",
		Method:   "eth_chainId",
		Count:    5,
		BytesIn:  100,
		Period:   period,
	})

	// Wait for async write
	time.Sleep(300 * time.Millisecond)

	rows, err := db.Pool().Query(ctx,
		`SELECT method, count, bytes_in FROM usage WHERE tenant_id = $1 AND period = $2`,
		"tenant-usage", period,
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

	recorder.RecordRequestLog(&model.RequestLog{
		TenantID:  "tenant-log",
		NetworkID: "net-1",
		Method:    "eth_call",
		Status:    "success",
		LatencyMS: 42,
		CacheHit:  true,
		BytesIn:   200,
		RequestID: "req-123",
	})

	time.Sleep(300 * time.Millisecond)

	ctx := context.Background()
	row := db.Pool().QueryRow(ctx,
		`SELECT method, status, latency_ms, cache_hit, request_id FROM request_logs WHERE tenant_id = $1`,
		"tenant-log",
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
	// Buffer of 1 ensures overflow on second enqueue
	recorder := New(db, logger.New(), 1)
	recorder.Start()
	defer recorder.Stop()

	// Fill the single slot
	recorder.RecordUsage(&model.Usage{TenantID: "overflow", Method: "m1", Count: 1, Period: time.Now()})
	// This one should drop without blocking or panicking
	recorder.RecordUsage(&model.Usage{TenantID: "overflow", Method: "m2", Count: 1, Period: time.Now()})

	// Should not panic or deadlock — test passes if we get here
}

func TestRecorder_DrainOnStop(t *testing.T) {
	db := setupTelemetryTestDB(t)
	recorder := New(db, logger.New(), 100)
	recorder.Start()

	ctx := context.Background()
	period := time.Now().Truncate(time.Minute)

	for i := 0; i < 10; i++ {
		recorder.RecordUsage(&model.Usage{
			TenantID: "drain-test",
			Method:   "eth_chainId",
			Count:    1,
			Period:   period,
		})
	}

	// Stop should drain remaining jobs
	recorder.Stop()

	// Verify at least some rows landed
	var count int64
	err := db.Pool().QueryRow(ctx,
		`SELECT COALESCE(SUM(count),0) FROM usage WHERE tenant_id = $1`,
		"drain-test",
	).Scan(&count)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, count, int64(1), "at least one usage record should have drained to DB")
}