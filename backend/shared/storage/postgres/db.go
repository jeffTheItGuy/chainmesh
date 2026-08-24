package postgres

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type DB struct {
	pool *pgxpool.Pool
}

func New(connString string) (*DB, error) {
	poolConfig, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("postgres parse config: %w", err)
	}

	// Apply connection pool limits from environment or sensible defaults.
	// Self-hosted deployments should tune these via env vars.
	poolConfig.MaxConns = int32(envInt("POSTGRES_MAX_CONNS", 20))
	poolConfig.MinConns = int32(envInt("POSTGRES_MIN_CONNS", 2))
	poolConfig.MaxConnLifetime = envDuration("POSTGRES_MAX_CONN_LIFETIME", 30*time.Minute)
	poolConfig.MaxConnIdleTime = envDuration("POSTGRES_MAX_CONN_IDLE_TIME", 10*time.Minute)
	poolConfig.HealthCheckPeriod = envDuration("POSTGRES_HEALTH_CHECK_PERIOD", 5*time.Minute)

	pool, err := pgxpool.NewWithConfig(context.Background(), poolConfig)
	if err != nil {
		return nil, fmt.Errorf("postgres connect: %w", err)
	}
	return &DB{pool: pool}, nil
}

func (d *DB) Close() { d.pool.Close() }

func (d *DB) Pool() *pgxpool.Pool { return d.pool }

func envInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return fallback
	}
	return n
}

func envDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		return fallback
	}
	return d
}
