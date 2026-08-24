package redis

import (
	"context"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

type Client struct {
	client *goredis.Client
}

type RateLimitStatus struct {
	LimitMinute       int    `json:"limit_minute"`
	RemainingMinute   int    `json:"remaining_minute"`
	ResetUnix         int64  `json:"reset_unix"`
	RetryAfterSeconds int    `json:"retry_after_seconds,omitempty"`
	RejectedLimit     string `json:"rejected_limit,omitempty"`
}

func New(addr string) *Client {
	return &Client{
		client: goredis.NewClient(&goredis.Options{Addr: addr}),
	}
}

func (c *Client) Close() error {
	return c.client.Close()
}

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

// CheckRateLimit preserves the old simple behavior for compatibility.
func (c *Client) CheckRateLimit(ctx context.Context, tenantID string, quotaRPM int) (bool, error) {
	key := fmt.Sprintf("ratelimit:%s:%s", tenantID, time.Now().Format("2006-01-02T15:04"))

	current, err := c.Incr(ctx, key)
	if err != nil {
		return false, err
	}

	if current == 1 {
		if err := c.Expire(ctx, key, 2*time.Minute); err != nil {
			return false, err
		}
	}

	return int(current) <= quotaRPM, nil
}

// atomicIncrExpireScript atomically increments a key and sets its expiry
// only if the key is new (INCR returned 1). This prevents the race condition
// where multiple concurrent requests create a key that never expires.
//
// KEYS[1] = rate limit key
// ARGV[1] = TTL in seconds
// Returns: the new counter value
const atomicIncrExpireScript = `
	local current = redis.call("INCR", KEYS[1])
	if current == 1 then
		redis.call("EXPIRE", KEYS[1], ARGV[1])
	end
	return current
`

// CheckRateLimits checks RPS, per-minute, and daily limits atomically.
// A quota of 0 means that specific limit is disabled.
func (c *Client) CheckRateLimits(
	ctx context.Context,
	tenantID string,
	quotaRPS int,
	quotaRPM int,
	quotaDaily int,
) (bool, RateLimitStatus, error) {
	now := time.Now()
	minuteReset := now.Truncate(time.Minute).Add(time.Minute)

	status := RateLimitStatus{
		LimitMinute:     quotaRPM,
		RemainingMinute: 0,
		ResetUnix:       minuteReset.Unix(),
	}

	if quotaRPS > 0 {
		rpsKey := fmt.Sprintf("rl:rps:%s:%d", tenantID, now.Unix())
		// 2-second window for RPS
		currentRPS, err := c.client.Eval(ctx, atomicIncrExpireScript, []string{rpsKey}, "2").Int64()
		if err != nil {
			return false, status, fmt.Errorf("rps incr failed: %w", err)
		}

		if int(currentRPS) > quotaRPS {
			status.RejectedLimit = "rps"
			status.RetryAfterSeconds = 1
			return false, status, nil
		}
	}

	if quotaRPM > 0 {
		minuteKey := fmt.Sprintf("rl:min:%s:%s", tenantID, now.Format("2006-01-02T15:04"))
		// 2-minute window for per-minute limit (generous cleanup margin)
		currentMinute, err := c.client.Eval(ctx, atomicIncrExpireScript, []string{minuteKey}, "120").Int64()
		if err != nil {
			return false, status, fmt.Errorf("rpm incr failed: %w", err)
		}

		remaining := quotaRPM - int(currentMinute)
		if remaining < 0 {
			remaining = 0
		}
		status.RemainingMinute = remaining

		if int(currentMinute) > quotaRPM {
			retry := time.Until(minuteReset).Seconds()
			if retry < 1 {
				retry = 1
			}

			status.RejectedLimit = "rpm"
			status.RetryAfterSeconds = int(retry)
			return false, status, nil
		}
	} else {
		status.LimitMinute = 0
		status.RemainingMinute = 0
	}

	if quotaDaily > 0 {
		dayKey := fmt.Sprintf("rl:day:%s:%s", tenantID, now.Format("2006-01-02"))
		// 25-hour window for daily limit (handles DST edge cases)
		currentDay, err := c.client.Eval(ctx, atomicIncrExpireScript, []string{dayKey}, "90000").Int64()
		if err != nil {
			return false, status, fmt.Errorf("daily incr failed: %w", err)
		}

		if int(currentDay) > quotaDaily {
			nextDay := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
			retry := time.Until(nextDay).Seconds()
			if retry < 1 {
				retry = 1
			}

			status.RejectedLimit = "daily"
			status.RetryAfterSeconds = int(retry)
			return false, status, nil
		}
	}

	return true, status, nil
}
