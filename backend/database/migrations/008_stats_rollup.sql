-- Pre-aggregated stats for faster dashboard queries.
--
-- This is intentionally denormalized and optimized for read-heavy stats.
-- p95 is stored per minute bucket; global p95 computed from this view is
-- approximate, not exact.

CREATE MATERIALIZED VIEW IF NOT EXISTS request_logs_rollup_1m AS
SELECT
    date_trunc('minute', created_at) AS bucket,
    COALESCE(network_id::text, '') AS network_id,
    method,
    status,
    cache_hit,
    COUNT(*) AS requests,
    COUNT(*) FILTER (WHERE status <> 'success') AS errors,
    COUNT(*) FILTER (WHERE cache_hit) AS cache_hits,
    COALESCE(AVG(latency_ms)::float8, 0) AS avg_latency_ms,
    COALESCE(percentile_cont(0.95) WITHIN GROUP (ORDER BY latency_ms)::float8, 0) AS p95_latency_ms
FROM request_logs
GROUP BY
    1, 2, 3, 4, 5
WITH NO DATA;

CREATE UNIQUE INDEX IF NOT EXISTS request_logs_rollup_1m_unique
ON request_logs_rollup_1m (
    bucket,
    network_id,
    method,
    status,
    cache_hit
);

CREATE INDEX IF NOT EXISTS request_logs_rollup_1m_bucket_idx
ON request_logs_rollup_1m (bucket);