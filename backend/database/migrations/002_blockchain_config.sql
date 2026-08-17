CREATE TABLE IF NOT EXISTS blockchain_configs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    rpc_endpoint_1 TEXT NOT NULL,
    rpc_endpoint_2 TEXT,
    chain_id TEXT,
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
