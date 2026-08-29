-- Multi-network support migration
-- Run this after 003_blockchain_config.up.sql

-- Add network reference to tenants
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS blockchain_network_id UUID REFERENCES blockchain_configs(id);

-- Add network reference to blocks
ALTER TABLE blocks ADD COLUMN IF NOT EXISTS network_id UUID REFERENCES blockchain_configs(id);

-- FIX: Drop the old single-column primary key so block numbers are no longer
-- globally unique. Then add a composite primary key so uniqueness is enforced
-- per-network instead.
ALTER TABLE blocks DROP CONSTRAINT IF EXISTS blocks_pkey;
ALTER TABLE blocks ADD PRIMARY KEY (number, network_id);

-- Drop the old unique constraint if it still exists from a previous partial run
ALTER TABLE blocks DROP CONSTRAINT IF EXISTS blocks_number_key;
ALTER TABLE blocks DROP CONSTRAINT IF EXISTS blocks_hash_key;

-- Index for faster block listing by network
CREATE INDEX IF NOT EXISTS idx_blocks_network_id ON blocks(network_id);