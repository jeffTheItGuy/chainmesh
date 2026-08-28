CREATE TABLE IF NOT EXISTS request_logs (
    id BIGSERIAL PRIMARY KEY,
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    network_id UUID REFERENCES blockchain_configs(id) ON DELETE SET NULL,
    method TEXT NOT NULL,
    status TEXT NOT NULL,
    latency_ms INTEGER NOT NULL DEFAULT 0,
    cache_hit BOOLEAN NOT NULL DEFAULT false,
    bytes_in INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_request_logs_created_at
    ON request_logs(created_at DESC);

CREATE INDEX IF NOT EXISTS idx_request_logs_tenant_created_at
    ON request_logs(tenant_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_request_logs_method
    ON request_logs(method);

CREATE INDEX IF NOT EXISTS idx_request_logs_status
    ON request_logs(status);

CREATE INDEX IF NOT EXISTS idx_request_logs_network_created_at
    ON request_logs(network_id, created_at DESC);