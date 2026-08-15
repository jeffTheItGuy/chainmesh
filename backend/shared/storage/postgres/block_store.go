package postgres

import (
	"context"

	"github.com/yourname/blockmesh/shared/model"
)

func (d *DB) StoreBlock(ctx context.Context, b *model.Block) error {
	_, err := d.pool.Exec(ctx,
		`INSERT INTO blocks (number, hash, parent_hash, timestamp, tx_count, raw_json)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 ON CONFLICT (number) DO NOTHING`,
		b.Number, b.Hash, b.ParentHash, b.Timestamp, b.TxCount, b.RawJSON,
	)
	return err
}

func (d *DB) GetLatestBlock(ctx context.Context) (*model.Block, error) {
	row := d.pool.QueryRow(ctx,
		`SELECT number, hash, parent_hash, timestamp, tx_count, raw_json 
		 FROM blocks ORDER BY number DESC LIMIT 1`,
	)
	b := &model.Block{}
	var rawJSON []byte
	err := row.Scan(&b.Number, &b.Hash, &b.ParentHash, &b.Timestamp, &b.TxCount, &rawJSON)
	if err != nil {
		return nil, err
	}
	b.RawJSON = rawJSON
	return b, nil
}

func (d *DB) ListBlocks(ctx context.Context, limit int) ([]model.Block, error) {
	rows, err := d.pool.Query(ctx,
		`SELECT number, hash, parent_hash, timestamp, tx_count 
		 FROM blocks ORDER BY number DESC LIMIT $1`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var blocks []model.Block
	for rows.Next() {
		var b model.Block
		err := rows.Scan(&b.Number, &b.Hash, &b.ParentHash, &b.Timestamp, &b.TxCount)
		if err != nil {
			return nil, err
		}
		blocks = append(blocks, b)
	}
	return blocks, nil
}
