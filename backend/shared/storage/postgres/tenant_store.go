package postgres

import (
	"context"
	"fmt"

	"github.com/jeffTheItGuy/chainmesh/shared/model"
	"github.com/jeffTheItGuy/chainmesh/shared/util"
)

func (d *DB) GetTenantByAPIKey(ctx context.Context, key string) (*model.Tenant, error) {
	prefix := util.APIKeyPrefix(key)

	rows, err := d.pool.Query(ctx,
		`
		SELECT id, key_hash, tenant_id
		FROM api_keys
		WHERE key_prefix = $1
		  AND revoked_at IS NULL
		`,
		prefix,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tenantID, keyID string
	for rows.Next() {
		var id, hash, tid string
		if err := rows.Scan(&id, &hash, &tid); err != nil {
			return nil, err
		}
		if util.VerifyAPIKey(key, hash) {
			tenantID = tid
			keyID = id
			break
		}
	}

	if tenantID == "" {
		return nil, fmt.Errorf("unauthorized")
	}

	// Update last_used_at for the matched key
	_, _ = d.pool.Exec(ctx,
		`UPDATE api_keys SET last_used_at = NOW() WHERE id = $1`,
		keyID,
	)

	return d.GetTenantByID(ctx, tenantID)
}

func (d *DB) GetTenantByID(ctx context.Context, id string) (*model.Tenant, error) {
	row := d.pool.QueryRow(ctx,
		`
SELECT
id,
name,
COALESCE(blockchain_network_id::text, ''),
COALESCE(quota_rpm, 0),
COALESCE(quota_rps, 0),
COALESCE(quota_daily, 0),
COALESCE(plan, 'free'),
created_at
FROM tenants
WHERE id = $1
`,
		id,
	)
	t := &model.Tenant{}
	err := row.Scan(
		&t.ID,
		&t.Name,
		&t.BlockchainNetworkID,
		&t.QuotaRPM,
		&t.QuotaRPS,
		&t.QuotaDaily,
		&t.Plan,
		&t.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return t, nil
}

func (d *DB) ListTenants(ctx context.Context) ([]model.Tenant, error) {
	rows, err := d.pool.Query(ctx,
		`
SELECT
id,
name,
COALESCE(blockchain_network_id::text, ''),
COALESCE(quota_rpm, 0),
COALESCE(quota_rps, 0),
COALESCE(quota_daily, 0),
COALESCE(plan, 'free'),
created_at
FROM tenants
ORDER BY created_at DESC
`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// FIX: Initialize as empty slice so JSON marshals to [] instead of null
	tenants := make([]model.Tenant, 0)
	for rows.Next() {
		var t model.Tenant
		if err := rows.Scan(
			&t.ID,
			&t.Name,
			&t.BlockchainNetworkID,
			&t.QuotaRPM,
			&t.QuotaRPS,
			&t.QuotaDaily,
			&t.Plan,
			&t.CreatedAt,
		); err != nil {
			return nil, err
		}
		tenants = append(tenants, t)
	}
	return tenants, nil
}

func (d *DB) CreateTenantWithKey(
	ctx context.Context,
	name string,
	blockchainNetworkID string,
	quotaRPM int,
	quotaRPS int,
	quotaDaily int,
	plan string,
	plainAPIKey string,
) (*model.Tenant, error) {
	if plan == "" {
		plan = "free"
	}
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var networkID any
	if blockchainNetworkID != "" {
		networkID = blockchainNetworkID
	}

	t := &model.Tenant{}
	err = tx.QueryRow(ctx,
		`
INSERT INTO tenants (
name,
blockchain_network_id,
quota_rpm,
quota_rps,
quota_daily,
plan
)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING
id,
name,
COALESCE(blockchain_network_id::text, ''),
quota_rpm,
quota_rps,
quota_daily,
plan,
created_at
`,
		name,
		networkID,
		quotaRPM,
		quotaRPS,
		quotaDaily,
		plan,
	).Scan(
		&t.ID,
		&t.Name,
		&t.BlockchainNetworkID,
		&t.QuotaRPM,
		&t.QuotaRPS,
		&t.QuotaDaily,
		&t.Plan,
		&t.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	keyHash := util.HashAPIKey(plainAPIKey)
	keyPrefix := util.APIKeyPrefix(plainAPIKey)
	_, err = tx.Exec(ctx,
		`
INSERT INTO api_keys (
tenant_id,
name,
key_hash,
key_prefix
)
VALUES ($1, $2, $3, $4)
`,
		t.ID,
		"default",
		keyHash,
		keyPrefix,
	)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return t, nil
}

func (d *DB) UpdateTenant(
	ctx context.Context,
	id string,
	name string,
	blockchainNetworkID string,
	quotaRPM int,
	quotaRPS int,
	quotaDaily int,
	plan string,
) error {
	var networkID any
	if blockchainNetworkID != "" {
		networkID = blockchainNetworkID
	}
	_, err := d.pool.Exec(ctx,
		`
UPDATE tenants
SET
name = $2,
blockchain_network_id = $3,
quota_rpm = $4,
quota_rps = $5,
quota_daily = $6,
plan = $7
WHERE id = $1
`,
		id,
		name,
		networkID,
		quotaRPM,
		quotaRPS,
		quotaDaily,
		plan,
	)
	return err
}

func (d *DB) DeleteTenant(ctx context.Context, id string) error {
	_, err := d.pool.Exec(ctx, `DELETE FROM tenants WHERE id = $1`, id)
	return err
}

func (d *DB) RotateAPIKey(ctx context.Context, tenantID string, plainAPIKey string) error {
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx,
		`
UPDATE api_keys
SET revoked_at = NOW()
WHERE tenant_id = $1
AND revoked_at IS NULL
`,
		tenantID,
	)
	if err != nil {
		return err
	}

	keyHash := util.HashAPIKey(plainAPIKey)
	keyPrefix := util.APIKeyPrefix(plainAPIKey)
	_, err = tx.Exec(ctx,
		`
INSERT INTO api_keys (
tenant_id,
name,
key_hash,
key_prefix
)
VALUES ($1, $2, $3, $4)
`,
		tenantID,
		"rotated",
		keyHash,
		keyPrefix,
	)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}
