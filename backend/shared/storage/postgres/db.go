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
	poolConfig.MaxConns = int32(envInt("POSTGRES_MAX_CONNS", 20))
	poolConfig.MinConns = int32(envInt("POSTGRES_MIN_CONNS", 2))
	poolConfig.MaxConnLifetime = envDuration("POSTGRES_MAX_CONN_LIFETIME", 30*time.Minute)
	poolConfig.MaxConnIdleTime = envDuration("POSTGRES_MAX_CONN_IDLE_TIME", 10*time.Minute)
	poolConfig.HealthCheckPeriod = envDuration("POSTGRES_HEALTH_CHECK_PERIOD", 5*time.Minute)

	var pool *pgxpool.Pool
	maxRetries := 10
	
	// Retry loop to survive Postgres init-script restarts and slow boot times.
	for attempt := 1; attempt <= maxRetries; attempt++ {
		pool, err = pgxpool.NewWithConfig(context.Background(), poolConfig)
		if err == nil {
			// pgxpool.NewWithConfig can return successfully even if the server 
			// is mid-restart. Ping forces an actual TCP connection + PG handshake.
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			pingErr := pool.Ping(ctx)
			cancel()
			
			if pingErr == nil {
				return &DB{pool: pool}, nil
			}
			pool.Close()
			err = pingErr
		}
		
		if attempt < maxRetries {
			time.Sleep(1 * time.Second)
		}
	}
	
	return nil, fmt.Errorf("postgres connect failed after %d attempts: %w", maxRetries, err)
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