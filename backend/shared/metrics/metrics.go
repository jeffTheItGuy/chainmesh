package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	RequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "blockmesh",
			Subsystem: "gateway",
			Name:      "requests_total",
			Help:      "Total gateway requests by network, method, status, and cache result.",
		},
		[]string{"network_id", "method", "status", "cache_hit"},
	)

	RequestDurationSeconds = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "blockmesh",
			Subsystem: "gateway",
			Name:      "request_duration_seconds",
			Help:      "Gateway request latency in seconds.",
			Buckets: []float64{
				0.005, 0.01, 0.025, 0.05, 0.1,
				0.25, 0.5, 1, 2.5, 5, 10,
			},
		},
		[]string{"network_id", "method", "status", "cache_hit"},
	)

	CacheHitsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "blockmesh",
			Subsystem: "gateway",
			Name:      "cache_hits_total",
			Help:      "Total Redis cache hits.",
		},
		[]string{"network_id", "method"},
	)

	CacheMissesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "blockmesh",
			Subsystem: "gateway",
			Name:      "cache_misses_total",
			Help:      "Total Redis cache misses for cacheable methods.",
		},
		[]string{"network_id", "method"},
	)

	RateLimitedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "blockmesh",
			Subsystem: "gateway",
			Name:      "rate_limited_total",
			Help:      "Total rate-limited requests by rejected limit.",
		},
		[]string{"limit"},
	)

	RateLimitErrorsTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: "blockmesh",
			Subsystem: "gateway",
			Name:      "rate_limit_errors_total",
			Help:      "Total errors encountered while checking rate limits.",
		},
	)

	UpstreamRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "blockmesh",
			Subsystem: "gateway",
			Name:      "upstream_requests_total",
			Help:      "Total upstream RPC requests.",
		},
		[]string{"network_id", "endpoint", "method", "status"},
	)

	UpstreamErrorsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "blockmesh",
			Subsystem: "gateway",
			Name:      "upstream_errors_total",
			Help:      "Total upstream RPC transport or invalid-response errors.",
		},
		[]string{"network_id", "endpoint", "method", "reason"},
	)

	UpstreamRequestDurationSeconds = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "blockmesh",
			Subsystem: "gateway",
			Name:      "upstream_request_duration_seconds",
			Help:      "Upstream RPC request latency in seconds.",
			Buckets: []float64{
				0.005, 0.01, 0.025, 0.05, 0.1,
				0.25, 0.5, 1, 2.5, 5, 10,
			},
		},
		[]string{"network_id", "endpoint", "method"},
	)

	NodeHealthy = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "blockmesh",
			Subsystem: "gateway",
			Name:      "node_healthy",
			Help:      "Health of each upstream blockchain node. 1 = healthy, 0 = unhealthy.",
		},
		[]string{"network_id", "endpoint"},
	)

	TelemetryWriteFailuresTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "blockmesh",
			Subsystem: "gateway",
			Name:      "telemetry_write_failures_total",
			Help:      "Total failed telemetry writes to Postgres.",
		},
		[]string{"kind"},
	)

	TelemetryDroppedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "blockmesh",
			Subsystem: "gateway",
			Name:      "telemetry_dropped_total",
			Help:      "Total telemetry records dropped after retries or queue overflow.",
		},
		[]string{"kind"},
	)
)
