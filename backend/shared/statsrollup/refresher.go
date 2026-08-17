package statsrollup

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/yourname/blockmesh/shared/storage/postgres"
)

// Refresher periodically refreshes the stats materialized view.
//
// This keeps dashboard queries fast as request_logs grows.
type Refresher struct {
	db       *postgres.DB
	log      *slog.Logger
	interval time.Duration

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func New(db *postgres.DB, log *slog.Logger, interval time.Duration) *Refresher {
	if interval <= 0 {
		interval = time.Minute
	}
	return &Refresher{
		db:       db,
		log:      log,
		interval: interval,
	}
}

func (r *Refresher) Start() {
	r.ctx, r.cancel = context.WithCancel(context.Background())
	r.wg.Add(1)
	go r.loop()
}

func (r *Refresher) Stop() {
	if r.cancel != nil {
		r.cancel()
	}
	r.wg.Wait()
}

func (r *Refresher) loop() {
	defer r.wg.Done()

	r.refresh()

	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case <-r.ctx.Done():
			return
		case <-ticker.C:
			r.refresh()
		}
	}
}

func (r *Refresher) refresh() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Prefer concurrent refresh once the view has been populated.
	_, err := r.db.Pool().Exec(ctx, `
		REFRESH MATERIALIZED VIEW CONCURRENTLY request_logs_rollup_1m
	`)
	if err == nil {
		return
	}

	r.log.Warn(
		"concurrent materialized view refresh failed, falling back to normal refresh",
		"err", err,
	)

	_, err = r.db.Pool().Exec(ctx, `
		REFRESH MATERIALIZED VIEW request_logs_rollup_1m
	`)
	if err != nil {
		r.log.Error("materialized view refresh failed", "err", err)
	}
}