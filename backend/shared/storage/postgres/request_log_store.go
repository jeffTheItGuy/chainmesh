package postgres

import (
	"context"

	"github.com/jeffTheItGuy/chainmesh/shared/model"
)

func (d *DB) RecordRequestLog(ctx context.Context, l *model.RequestLog) error {
	var networkID any
	if l.NetworkID != "" {
		networkID = l.NetworkID
	}

	var requestID any
	if l.RequestID != "" {
		requestID = l.RequestID
	}

	_, err := d.pool.Exec(ctx,
		`
		INSERT INTO request_logs (
			tenant_id,
			network_id,
			method,
			status,
			latency_ms,
			cache_hit,
			bytes_in,
			request_id
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		`,
		l.TenantID,
		networkID,
		l.Method,
		l.Status,
		l.LatencyMS,
		l.CacheHit,
		l.BytesIn,
		requestID,
	)

	return err
}
