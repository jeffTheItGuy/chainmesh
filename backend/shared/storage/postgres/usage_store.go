package postgres

import (
	"context"
	"time"

	"github.com/yourname/blockmesh/shared/model"
)

func (d *DB) RecordUsage(ctx context.Context, u *model.Usage) error {
	_, err := d.pool.Exec(ctx,
		`INSERT INTO usage (tenant_id, method, count, bytes_in, period)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (tenant_id, method, period)
		 DO UPDATE SET count = usage.count + $3, bytes_in = usage.bytes_in + $4`,
		u.TenantID, u.Method, u.Count, u.BytesIn, u.Period,
	)
	return err
}

func (d *DB) GetDailyUsage(ctx context.Context, tenantID string, day time.Time) ([]model.Usage, error) {
	rows, err := d.pool.Query(ctx,
		`SELECT tenant_id, method, count, bytes_in, period
		 FROM usage
		 WHERE tenant_id = $1 AND DATE(period) = DATE($2)`,
		tenantID, day,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.Usage
	for rows.Next() {
		var u model.Usage
		rows.Scan(&u.TenantID, &u.Method, &u.Count, &u.BytesIn, &u.Period)
		out = append(out, u)
	}
	return out, nil
}
