package postgres

import (
	"context"
	"time"

	"github.com/yourname/blockmesh/shared/model"
)

func (d *DB) GetStatsSummary(
	ctx context.Context,
	from time.Time,
	rangeName string,
) (*model.StatsSummary, error) {
	// Use the rollup for older ranges.
	//
	// For very recent data, raw request_logs is still more complete because
	// the materialized view refreshes on a periodic schedule.
	recentCutoff := time.Now().Add(-5 * time.Minute)
	if from.Before(recentCutoff) {
		summary, err := d.getStatsSummaryFromRollup(ctx, from, rangeName)
		if err == nil {
			return summary, nil
		}

		// Fall back to raw logs if the rollup is unavailable.
	}

	return d.getStatsSummaryFromRaw(ctx, from, rangeName)
}

func (d *DB) getStatsSummaryFromRaw(
	ctx context.Context,
	from time.Time,
	rangeName string,
) (*model.StatsSummary, error) {
	summary := &model.StatsSummary{
		Range:       rangeName,
		From:        from,
		To:          time.Now(),
		TopMethods:  make([]model.StatsCount, 0),
		TopStatuses: make([]model.StatsCount, 0),
		TopNetworks: make([]model.StatsCount, 0),
		Series:      make([]model.StatsSeriesPoint, 0),
	}

	err := d.pool.QueryRow(ctx,
		`
		SELECT
			COUNT(*),
			COUNT(*) FILTER (WHERE status = 'success'),
			COUNT(*) FILTER (WHERE status <> 'success'),
			COUNT(*) FILTER (WHERE cache_hit),
			COALESCE(AVG(latency_ms)::float8, 0),
			COALESCE(percentile_cont(0.95) WITHIN GROUP (ORDER BY latency_ms)::float8, 0)
		FROM request_logs
		WHERE created_at >= $1
		`,
		from,
	).Scan(
		&summary.Totals.Requests,
		&summary.Totals.Success,
		&summary.Totals.Errors,
		&summary.Totals.CacheHits,
		&summary.Latency.AvgMS,
		&summary.Latency.P95MS,
	)
	if err != nil {
		return nil, err
	}

	summary.Totals.CacheMisses = summary.Totals.Requests - summary.Totals.CacheHits

	rows, err := d.pool.Query(ctx,
		`
		SELECT method, COUNT(*)
		FROM request_logs
		WHERE created_at >= $1
		  AND method <> ''
		GROUP BY method
		ORDER BY count DESC
		LIMIT 5
		`,
		from,
	)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var item model.StatsCount
		if err := rows.Scan(&item.Name, &item.Count); err != nil {
			rows.Close()
			return nil, err
		}
		summary.TopMethods = append(summary.TopMethods, item)
	}
	rows.Close()

	rows, err = d.pool.Query(ctx,
		`
		SELECT status, COUNT(*)
		FROM request_logs
		WHERE created_at >= $1
		GROUP BY status
		ORDER BY count DESC
		LIMIT 5
		`,
		from,
	)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var item model.StatsCount
		if err := rows.Scan(&item.Name, &item.Count); err != nil {
			rows.Close()
			return nil, err
		}
		summary.TopStatuses = append(summary.TopStatuses, item)
	}
	rows.Close()

	rows, err = d.pool.Query(ctx,
		`
		SELECT
			COALESCE(c.name, rl.network_id::text, 'unknown') AS network,
			COUNT(*)
		FROM request_logs rl
		LEFT JOIN blockchain_configs c ON rl.network_id = c.id
		WHERE rl.created_at >= $1
		GROUP BY 1
		ORDER BY count DESC
		LIMIT 5
		`,
		from,
	)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var item model.StatsCount
		if err := rows.Scan(&item.Name, &item.Count); err != nil {
			rows.Close()
			return nil, err
		}
		summary.TopNetworks = append(summary.TopNetworks, item)
	}
	rows.Close()

	var seriesQuery string
	if rangeName == "24h" {
		seriesQuery = `
			SELECT
				date_trunc('hour', created_at) AS bucket,
				COUNT(*),
				COUNT(*) FILTER (WHERE status <> 'success'),
				COUNT(*) FILTER (WHERE cache_hit)
			FROM request_logs
			WHERE created_at >= $1
			GROUP BY bucket
			ORDER BY bucket
		`
	} else {
		seriesQuery = `
			SELECT
				date_trunc('minute', created_at) AS bucket,
				COUNT(*),
				COUNT(*) FILTER (WHERE status <> 'success'),
				COUNT(*) FILTER (WHERE cache_hit)
			FROM request_logs
			WHERE created_at >= $1
			GROUP BY bucket
			ORDER BY bucket
		`
	}

	rows, err = d.pool.Query(ctx, seriesQuery, from)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var point model.StatsSeriesPoint
		if err := rows.Scan(
			&point.Time,
			&point.Requests,
			&point.Errors,
			&point.CacheHits,
		); err != nil {
			return nil, err
		}
		summary.Series = append(summary.Series, point)
	}

	return summary, nil
}

func (d *DB) getStatsSummaryFromRollup(
	ctx context.Context,
	from time.Time,
	rangeName string,
) (*model.StatsSummary, error) {
	summary := &model.StatsSummary{
		Range:       rangeName,
		From:        from,
		To:          time.Now(),
		TopMethods:  make([]model.StatsCount, 0),
		TopStatuses: make([]model.StatsCount, 0),
		TopNetworks: make([]model.StatsCount, 0),
		Series:      make([]model.StatsSeriesPoint, 0),
	}

	err := d.pool.QueryRow(ctx,
		`
		SELECT
			COALESCE(SUM(requests), 0),
			COALESCE(SUM(errors), 0),
			COALESCE(SUM(cache_hits), 0),
			COALESCE(SUM(avg_latency_ms * requests) / NULLIF(SUM(requests), 0), 0),
			COALESCE(MAX(p95_latency_ms), 0)
		FROM request_logs_rollup_1m
		WHERE bucket >= $1
		`,
		from,
	).Scan(
		&summary.Totals.Requests,
		&summary.Totals.Errors,
		&summary.Totals.CacheHits,
		&summary.Latency.AvgMS,
		&summary.Latency.P95MS,
	)
	if err != nil {
		return nil, err
	}

	summary.Totals.Success = summary.Totals.Requests - summary.Totals.Errors
	summary.Totals.CacheMisses = summary.Totals.Requests - summary.Totals.CacheHits

	rows, err := d.pool.Query(ctx,
		`
		SELECT method, COALESCE(SUM(requests), 0)
		FROM request_logs_rollup_1m
		WHERE bucket >= $1
		  AND method <> ''
		GROUP BY method
		ORDER BY count DESC
		LIMIT 5
		`,
		from,
	)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var item model.StatsCount
		if err := rows.Scan(&item.Name, &item.Count); err != nil {
			rows.Close()
			return nil, err
		}
		summary.TopMethods = append(summary.TopMethods, item)
	}
	rows.Close()

	rows, err = d.pool.Query(ctx,
		`
		SELECT status, COALESCE(SUM(requests), 0)
		FROM request_logs_rollup_1m
		WHERE bucket >= $1
		GROUP BY status
		ORDER BY count DESC
		LIMIT 5
		`,
		from,
	)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var item model.StatsCount
		if err := rows.Scan(&item.Name, &item.Count); err != nil {
			rows.Close()
			return nil, err
		}
		summary.TopStatuses = append(summary.TopStatuses, item)
	}
	rows.Close()

	rows, err = d.pool.Query(ctx,
		`
		SELECT
			COALESCE(NULLIF(c.name, ''), NULLIF(r.network_id, ''), 'unknown') AS network,
			COALESCE(SUM(r.requests), 0)
		FROM request_logs_rollup_1m r
		LEFT JOIN blockchain_configs c ON c.id::text = r.network_id
		WHERE r.bucket >= $1
		GROUP BY 1
		ORDER BY count DESC
		LIMIT 5
		`,
		from,
	)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var item model.StatsCount
		if err := rows.Scan(&item.Name, &item.Count); err != nil {
			rows.Close()
			return nil, err
		}
		summary.TopNetworks = append(summary.TopNetworks, item)
	}
	rows.Close()

	var seriesQuery string
	if rangeName == "24h" {
		seriesQuery = `
			SELECT
				date_trunc('hour', bucket) AS bucket,
				COALESCE(SUM(requests), 0),
				COALESCE(SUM(errors), 0),
				COALESCE(SUM(cache_hits), 0)
			FROM request_logs_rollup_1m
			WHERE bucket >= $1
			GROUP BY bucket
			ORDER BY bucket
		`
	} else {
		seriesQuery = `
			SELECT
				bucket,
				COALESCE(SUM(requests), 0),
				COALESCE(SUM(errors), 0),
				COALESCE(SUM(cache_hits), 0)
			FROM request_logs_rollup_1m
			WHERE bucket >= $1
			GROUP BY bucket
			ORDER BY bucket
		`
	}

	rows, err = d.pool.Query(ctx, seriesQuery, from)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var point model.StatsSeriesPoint
		if err := rows.Scan(
			&point.Time,
			&point.Requests,
			&point.Errors,
			&point.CacheHits,
		); err != nil {
			return nil, err
		}
		summary.Series = append(summary.Series, point)
	}

	return summary, nil
}