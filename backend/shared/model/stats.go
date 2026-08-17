package model

import "time"

type StatsTotals struct {
	Requests    int64 `json:"requests"`
	Success     int64 `json:"success"`
	Errors      int64 `json:"errors"`
	CacheHits   int64 `json:"cache_hits"`
	CacheMisses int64 `json:"cache_misses"`
}

type StatsLatency struct {
	AvgMS float64 `json:"avg_ms"`
	P95MS float64 `json:"p95_ms"`
}

type StatsCount struct {
	Name  string `json:"name"`
	Count int64  `json:"count"`
}

type StatsSeriesPoint struct {
	Time      time.Time `json:"time"`
	Requests  int64     `json:"requests"`
	Errors    int64     `json:"errors"`
	CacheHits int64     `json:"cache_hits"`
}

type StatsSummary struct {
	Range       string             `json:"range"`
	From        time.Time          `json:"from"`
	To          time.Time          `json:"to"`
	Totals      StatsTotals        `json:"totals"`
	Latency     StatsLatency       `json:"latency"`
	TopMethods  []StatsCount       `json:"top_methods"`
	TopStatuses []StatsCount       `json:"top_statuses"`
	TopNetworks []StatsCount       `json:"top_networks"`
	Series      []StatsSeriesPoint `json:"series"`
}