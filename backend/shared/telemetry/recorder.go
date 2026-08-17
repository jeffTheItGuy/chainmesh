package telemetry

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/yourname/blockmesh/shared/metrics"
	"github.com/yourname/blockmesh/shared/model"
	"github.com/yourname/blockmesh/shared/storage/postgres"
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

func (r *Recorder) Stop() {
	if r.cancel != nil {
		r.cancel()
	}
	r.wg.Wait()
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
			return
		case j := <-r.queue:
			r.process(j)
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