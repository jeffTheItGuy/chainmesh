CREATE TABLE IF NOT EXISTS tenants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    api_key TEXT UNIQUE NOT NULL,
    quota_rpm INT NOT NULL DEFAULT 100,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS usage (
    tenant_id UUID NOT NULL,
    method TEXT NOT NULL,
    count BIGINT NOT NULL DEFAULT 0,
    bytes_in BIGINT NOT NULL DEFAULT 0,
    period TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (tenant_id, method, period)
);

INSERT INTO tenants (name, api_key, quota_rpm) VALUES
('demo', 'demo-key', 1000)
ON CONFLICT DO NOTHING;
