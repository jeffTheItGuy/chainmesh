package postgres

import (
	"context"

	"github.com/yourname/blockmesh/shared/model"
)

func (d *DB) GetTenantByAPIKey(ctx context.Context, key string) (*model.Tenant, error) {
	row := d.pool.QueryRow(ctx,
		`SELECT id, name, api_key, quota_rpm, created_at FROM tenants WHERE api_key = $1`,
		key,
	)
	t := &model.Tenant{}
	err := row.Scan(&t.ID, &t.Name, &t.APIKey, &t.QuotaRPM, &t.CreatedAt)
	return t, err
}

func (d *DB) GetTenantByID(ctx context.Context, id string) (*model.Tenant, error) {
	row := d.pool.QueryRow(ctx,
		`SELECT id, name, api_key, quota_rpm, created_at FROM tenants WHERE id = $1`,
		id,
	)
	t := &model.Tenant{}
	err := row.Scan(&t.ID, &t.Name, &t.APIKey, &t.QuotaRPM, &t.CreatedAt)
	return t, err
}

func (d *DB) ListTenants(ctx context.Context) ([]model.Tenant, error) {
	rows, err := d.pool.Query(ctx,
		`SELECT id, name, api_key, quota_rpm, created_at FROM tenants ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tenants []model.Tenant
	for rows.Next() {
		var t model.Tenant
		if err := rows.Scan(&t.ID, &t.Name, &t.APIKey, &t.QuotaRPM, &t.CreatedAt); err != nil {
			return nil, err
		}
		tenants = append(tenants, t)
	}
	return tenants, nil
}

func (d *DB) CreateTenant(ctx context.Context, name, apiKey string, quotaRPM int) (*model.Tenant, error) {
	row := d.pool.QueryRow(ctx,
		`INSERT INTO tenants (name, api_key, quota_rpm) VALUES ($1, $2, $3)
		 RETURNING id, name, api_key, quota_rpm, created_at`,
		name, apiKey, quotaRPM,
	)
	t := &model.Tenant{}
	err := row.Scan(&t.ID, &t.Name, &t.APIKey, &t.QuotaRPM, &t.CreatedAt)
	return t, err
}
