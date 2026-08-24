package telemetry

import (
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
	return db
}

func TestRecorder_StartStopNoGoroutineLeak(t *testing.T) {
	db := setupTelemetryTestDB(t)

	// Let any background runtime goroutines settle
	time.Sleep(100 * time.Millisecond)
	runtime.GC()
	baseline := runtime.NumGoroutine()

	recorder := New(db, logger.New(), 100)
	recorder.Start()
	time.Sleep(50 * time.Millisecond) // let worker start

	recorder.Stop()
	time.Sleep(100 * time.Millisecond) // let worker exit
	runtime.GC()

	after := runtime.NumGoroutine()
	// Allow a small margin for runtime noise
	assert.LessOrEqual(t, after, baseline+2, "goroutine count should return to baseline after Stop")
}

func TestRecorder_StopIdempotency(t *testing.T) {
	db := setupTelemetryTestDB(t)
	recorder := New(db, logger.New(), 100)

	recorder.Start()
	time.Sleep(50 * time.Millisecond)

	// Must not panic or deadlock
	recorder.Stop()
	recorder.Stop()
	recorder.Stop()

	// After stop, enqueue should drop cleanly without spawning new goroutines
	recorder.RecordUsage(&model.Usage{
		TenantID: "t1",
		Method:   "eth_chainId",
		Count:    1,
		Period:   time.Now().Truncate(time.Minute),
	})
}
