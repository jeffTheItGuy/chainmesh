package postgres

import (
	"context"

	"github.com/jeffTheItGuy/chainmesh/shared/model"
)

func (d *DB) RecordAuditLog(ctx context.Context, log *model.AuditLog) error {
	_, err := d.pool.Exec(ctx,
		`
		INSERT INTO audit_logs (
			actor,
			action,
			resource_type,
			resource_id,
			details,
			ip_address,
			user_agent,
			created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		`,
		log.Actor,
		log.Action,
		log.ResourceType,
		log.ResourceID,
		log.Details,
		log.IPAddress,
		log.UserAgent,
		log.CreatedAt,
	)
	return err
}

func (d *DB) ListAuditLogs(ctx context.Context, limit, offset int) ([]model.AuditLog, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 1000 {
		limit = 1000
	}
	if offset < 0 {
		offset = 0
	}

	rows, err := d.pool.Query(ctx,
		`
		SELECT id, actor, action, resource_type, resource_id, details, ip_address::text, user_agent, created_at
		FROM audit_logs
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
		`,
		limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]model.AuditLog, 0)
	for rows.Next() {
		var l model.AuditLog
		var details []byte
		err := rows.Scan(&l.ID, &l.Actor, &l.Action, &l.ResourceType, &l.ResourceID, &details, &l.IPAddress, &l.UserAgent, &l.CreatedAt)
		if err != nil {
			return nil, err
		}
		if len(details) > 0 {
			l.Details = details
		}
		out = append(out, l)
	}
	return out, nil
}
