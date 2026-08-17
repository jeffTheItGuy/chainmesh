package postgres

import (
	"context"
	"github.com/yourname/blockmesh/shared/model"
)

func (d *DB) ListBlockchainConfigs(ctx context.Context) ([]model.BlockchainConfig, error) {
	rows, err := d.pool.Query(ctx,
		`SELECT id, name, rpc_endpoint_1, rpc_endpoint_2, chain_id, enabled, created_at, updated_at
FROM blockchain_configs ORDER BY name`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var configs []model.BlockchainConfig
	for rows.Next() {
		var c model.BlockchainConfig
		if err := rows.Scan(&c.ID, &c.Name, &c.RPCEndpoint1, &c.RPCEndpoint2, &c.ChainID, &c.Enabled, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		configs = append(configs, c)
	}
	return configs, nil
}

func (d *DB) GetBlockchainConfig(ctx context.Context, id string) (*model.BlockchainConfig, error) {
	row := d.pool.QueryRow(ctx,
		`SELECT id, name, rpc_endpoint_1, rpc_endpoint_2, chain_id, enabled, created_at, updated_at
FROM blockchain_configs WHERE id = $1`,
		id,
	)
	c := &model.BlockchainConfig{}
	err := row.Scan(&c.ID, &c.Name, &c.RPCEndpoint1, &c.RPCEndpoint2, &c.ChainID, &c.Enabled, &c.CreatedAt, &c.UpdatedAt)
	return c, err
}

func (d *DB) GetDefaultBlockchainConfig(ctx context.Context) (*model.BlockchainConfig, error) {
	row := d.pool.QueryRow(ctx,
		`SELECT id, name, rpc_endpoint_1, rpc_endpoint_2, chain_id, enabled, created_at, updated_at
FROM blockchain_configs WHERE enabled = true ORDER BY created_at LIMIT 1`,
	)
	c := &model.BlockchainConfig{}
	err := row.Scan(&c.ID, &c.Name, &c.RPCEndpoint1, &c.RPCEndpoint2, &c.ChainID, &c.Enabled, &c.CreatedAt, &c.UpdatedAt)
	return c, err
}

func (d *DB) SaveBlockchainConfig(ctx context.Context, cfg *model.BlockchainConfig) (*model.BlockchainConfig, error) {
	row := d.pool.QueryRow(ctx,
		`INSERT INTO blockchain_configs (name, rpc_endpoint_1, rpc_endpoint_2, chain_id, enabled)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, name, rpc_endpoint_1, rpc_endpoint_2, chain_id, enabled, created_at, updated_at`,
		cfg.Name, cfg.RPCEndpoint1, cfg.RPCEndpoint2, cfg.ChainID, cfg.Enabled,
	)
	c := &model.BlockchainConfig{}
	err := row.Scan(&c.ID, &c.Name, &c.RPCEndpoint1, &c.RPCEndpoint2, &c.ChainID, &c.Enabled, &c.CreatedAt, &c.UpdatedAt)
	return c, err
}

func (d *DB) UpdateBlockchainConfig(ctx context.Context, cfg *model.BlockchainConfig) error {
	_, err := d.pool.Exec(ctx,
		`UPDATE blockchain_configs
SET name = $1, rpc_endpoint_1 = $2, rpc_endpoint_2 = $3, chain_id = $4, enabled = $5, updated_at = NOW()
WHERE id = $6`,
		cfg.Name, cfg.RPCEndpoint1, cfg.RPCEndpoint2, cfg.ChainID, cfg.Enabled, cfg.ID,
	)
	return err
}

func (d *DB) DeleteBlockchainConfig(ctx context.Context, id string) error {
	// FIX: Use a transaction to unlink dependent records first, avoiding FK violations.
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// 1. Unlink tenants referencing this network
	_, err = tx.Exec(ctx, `UPDATE tenants SET blockchain_network_id = NULL WHERE blockchain_network_id = $1`, id)
	if err != nil {
		return err
	}

	// 2. Unlink blocks referencing this network
	_, err = tx.Exec(ctx, `UPDATE blocks SET network_id = NULL WHERE network_id = $1`, id)
	if err != nil {
		return err
	}

	// 3. Now it is safe to delete the network config
	_, err = tx.Exec(ctx, `DELETE FROM blockchain_configs WHERE id = $1`, id)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}