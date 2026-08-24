package model

import "time"

type RequestLog struct {
	ID        int64     `json:"id"`
	TenantID  string    `json:"tenant_id"`
	NetworkID string    `json:"network_id,omitempty"`
	Method    string    `json:"method"`
	Status    string    `json:"status"`
	LatencyMS int64     `json:"latency_ms"`
	CacheHit  bool      `json:"cache_hit"`
	BytesIn   int64     `json:"bytes_in"`
	RequestID string    `json:"request_id,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}
