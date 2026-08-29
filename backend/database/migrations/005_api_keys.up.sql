-- Make old plaintext tenant API key optional.
-- Existing deployments likely have tenants.api_key from 001_init.
ALTER TABLE tenants ALTER COLUMN api_key DROP NOT NULL;

CREATE TABLE IF NOT EXISTS api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name TEXT NOT NULL DEFAULT 'default',
    key_hash TEXT NOT NULL UNIQUE,
    key_prefix TEXT NOT NULL,
    revoked_at TIMESTAMPTZ,
    last_used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_api_keys_tenant_id ON api_keys(tenant_id);
CREATE INDEX IF NOT EXISTS idx_api_keys_key_hash ON api_keys(key_hash);