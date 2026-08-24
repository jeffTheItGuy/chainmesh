package telemetry

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/jeffTheItGuy/chainmesh/shared/metrics"
	"github.com/jeffTheItGuy/chainmesh/shared/model"
	"github.com/jeffTheItGuy/chainmesh/shared/storage/postgres"
)

type kind string

const (
	kindUsage      kind = "usage"
	kindRequestLog kind = "request_log"
)

type job struct {
	kind       kind
	usage      *model.Usage
	requestLog *model.RequestLog
}

// Recorder is a bounded async telemetry writer.
//
// It prevents short Postgres outages from synchronously blocking gateway
// requests, and it exposes dropped-write counters via Prometheus.
type Recorder struct {
	db    *postgres.DB
	log   *slog.Logger
	queue chan job

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func New(db *postgres.DB, log *slog.Logger, bufferSize int) *Recorder {
	if bufferSize <= 0 {
		bufferSize = 4096
	}
	return &Recorder{
		db:    db,
		log:   log,
		queue: make(chan job, bufferSize),
	}
}

func (r *Recorder) Start() {
	r.ctx, r.cancel = context.WithCancel(context.Background())
	r.wg.Add(1)
	go r.worker()
}

// Stop gracefully shuts down the telemetry worker with a bounded timeout.
// It signals cancellation, drains the queue, and waits up to the configured
// timeout for in-flight writes to complete. Any remaining jobs are dropped.
func (r *Recorder) Stop() {
	if r.cancel == nil {
		return
	}

	// Signal the worker to stop accepting new jobs and finish current ones
	r.cancel()

	// Wait for worker to finish with a timeout to prevent indefinite hangs
	// during Postgres outages or slow writes.
	timeout := envDuration("TELEMETRY_SHUTDOWN_TIMEOUT", 5*time.Second)

	done := make(chan struct{})
	go func() {
		r.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		r.log.Info("telemetry worker stopped cleanly")
	case <-time.After(timeout):
		r.log.Warn("telemetry worker shutdown timed out", "timeout", timeout, "dropped_jobs", len(r.queue))
	}
}

func (r *Recorder) RecordUsage(u *model.Usage) {
	r.enqueue(job{
		kind:  kindUsage,
		usage: u,
	})
}

func (r *Recorder) RecordRequestLog(l *model.RequestLog) {
	r.enqueue(job{
		kind:       kindRequestLog,
		requestLog: l,
	})
}

func (r *Recorder) enqueue(j job) {
	if r.ctx == nil || r.ctx.Err() != nil {
		metrics.TelemetryDroppedTotal.WithLabelValues(string(j.kind)).Inc()
		return
	}

	select {
	case r.queue <- j:
	default:
		metrics.TelemetryDroppedTotal.WithLabelValues(string(j.kind)).Inc()
	}
}

func (r *Recorder) worker() {
	defer r.wg.Done()

	for {
		select {
		case <-r.ctx.Done():
			// Drain remaining queue before exiting
			r.drain()
			return
		case j := <-r.queue:
			r.process(j)
		}
	}
}

// drain processes any remaining jobs in the queue after cancellation.
// This gives in-flight telemetry a chance to land before shutdown.
func (r *Recorder) drain() {
	drainTimeout := envDuration("TELEMETRY_DRAIN_TIMEOUT", 2*time.Second)
	deadline := time.After(drainTimeout)

	for {
		select {
		case j := <-r.queue:
			r.process(j)
		case <-deadline:
			remaining := len(r.queue)
			if remaining > 0 {
				r.log.Warn("telemetry drain incomplete", "remaining_jobs", remaining)
			}
			return
		default:
			// Queue is empty and no deadline hit — we're done
			if len(r.queue) == 0 {
				return
			}
			// Brief pause to avoid busy-waiting
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func (r *Recorder) process(j job) {
	const maxAttempts = 3

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		writeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

		var err error
		switch j.kind {
		case kindUsage:
			err = r.db.RecordUsage(writeCtx, j.usage)
		case kindRequestLog:
			err = r.db.RecordRequestLog(writeCtx, j.requestLog)
		}

		cancel()

		if err == nil {
			return
		}

		metrics.TelemetryWriteFailuresTotal.WithLabelValues(string(j.kind)).Inc()
		r.log.Warn(
			"telemetry write failed",
			"kind", string(j.kind),
			"attempt", attempt,
			"err", err,
		)

		if attempt == maxAttempts {
			break
		}

		backoff := time.Duration(50*attempt) * time.Millisecond
		select {
		case <-r.ctx.Done():
			return
		case <-time.After(backoff):
		}
	}

	metrics.TelemetryDroppedTotal.WithLabelValues(string(j.kind)).Inc()
	r.log.Error(
		"telemetry write dropped after retries",
		"kind", string(j.kind),
	)
}

func envDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		return fallback
	}
	return d
}
