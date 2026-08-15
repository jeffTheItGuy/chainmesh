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
