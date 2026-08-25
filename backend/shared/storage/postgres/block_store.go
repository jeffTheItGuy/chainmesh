// backend/shared/storage/postgres/block_store.go
package postgres

import (
	"context"

	"github.com/jeffTheItGuy/chainmesh/shared/model"
)

func (d *DB) StoreBlock(ctx context.Context, b *model.Block) error {
	_, err := d.pool.Exec(ctx,
		`INSERT INTO blocks (number, hash, parent_hash, timestamp, tx_count, network_id, raw_json)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 ON CONFLICT (number, network_id) DO NOTHING`,
		b.Number, b.Hash, b.ParentHash, b.Timestamp, b.TxCount, b.NetworkID, b.RawJSON,
	)
	return err
}

func (d *DB) GetLatestBlock(ctx context.Context, networkID string) (*model.Block, error) {
	row := d.pool.QueryRow(ctx,
		`SELECT number, hash, parent_hash, timestamp, tx_count, network_id, raw_json
		 FROM blocks WHERE network_id = $1 ORDER BY number DESC LIMIT 1`,
		networkID,
	)
	b := &model.Block{}
	var rawJSON []byte
	err := row.Scan(&b.Number, &b.Hash, &b.ParentHash, &b.Timestamp, &b.TxCount, &b.NetworkID, &rawJSON)
	if err != nil {
		return nil, err
	}
	b.RawJSON = rawJSON
	return b, nil
}

func (d *DB) ListBlocks(ctx context.Context, limit int) ([]model.Block, error) {
	rows, err := d.pool.Query(ctx,
		`SELECT b.number, b.hash, b.parent_hash, b.timestamp, b.tx_count, b.network_id, COALESCE(c.name, '') as network_name
		 FROM blocks b
		 LEFT JOIN blockchain_configs c ON b.network_id = c.id
		 ORDER BY b.number DESC LIMIT $1`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	blocks := make([]model.Block, 0, limit)
	for rows.Next() {
		var b model.Block
		err := rows.Scan(&b.Number, &b.Hash, &b.ParentHash, &b.Timestamp, &b.TxCount, &b.NetworkID, &b.NetworkName)
		if err != nil {
			return nil, err
		}
		blocks = append(blocks, b)
	}
	return blocks, nil
}