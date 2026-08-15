package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type DB struct {
	pool *pgxpool.Pool
}

func New(connString string) (*DB, error) {
	pool, err := pgxpool.New(context.Background(), connString)
	if err != nil {
		return nil, fmt.Errorf("postgres connect: %w", err)
	}
	return &DB{pool: pool}, nil
}

func (d *DB) Close() { d.pool.Close() }

func (d *DB) Pool() *pgxpool.Pool { return d.pool }
