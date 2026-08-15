package redis

import (
	"context"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

type Client struct {
	client *goredis.Client
}

func New(addr string) *Client {
	return &Client{
		client: goredis.NewClient(&goredis.Options{Addr: addr}),
	}
}

func (c *Client) Close() error { return c.client.Close() }

func (c *Client) Get(ctx context.Context, key string) (string, error) {
	return c.client.Get(ctx, key).Result()
}

func (c *Client) Set(ctx context.Context, key string, val string, ttl time.Duration) error {
	return c.client.Set(ctx, key, val, ttl).Err()
}

func (c *Client) Incr(ctx context.Context, key string) (int64, error) {
	return c.client.Incr(ctx, key).Result()
}

func (c *Client) Expire(ctx context.Context, key string, ttl time.Duration) error {
	return c.client.Expire(ctx, key, ttl).Err()
}
