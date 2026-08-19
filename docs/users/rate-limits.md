# Rate Limits

BlockMesh enforces per-tenant rate limits to ensure fair resource sharing and protect upstream nodes. This document explains the limit types, how they work, and how to handle exceeded quotas.

---

## Limit Types

Each tenant has three independent quotas:

| Limit | Window | Purpose |
|-------|--------|---------|
| **RPS** (Requests Per Second) | 1 second | Burst protection |
| **RPM** (Requests Per Minute) | 1 minute | Sustained throughput |
| **Daily** | 24 hours | Monthly billing alignment |

A quota of `0` means that limit is disabled (unlimited).

---

## How Limits Are Enforced

BlockMesh uses Redis-backed counters with atomic operations to prevent race conditions:

1. Each request increments a Redis counter for the current time window
2. If the counter exceeds the quota, the request is rejected with `429 Too Many Requests`
3. Counters expire automatically after their window (with a safety margin)

The Lua script used for atomic increment + expiry:

```lua
local current = redis.call("INCR", KEYS[1])
if current == 1 then
    redis.call("EXPIRE", KEYS[1], ARGV[1])
end
return current
```

This prevents the race condition where two concurrent requests both see `current=0` and only one sets the expiry.

---

## Response Headers

Every authenticated response includes rate limit headers:

| Header | Description |
|--------|-------------|
| `X-RateLimit-Limit-Minute` | Your RPM quota |
| `X-RateLimit-Remaining-Minute` | Requests remaining in current minute |
| `X-RateLimit-Reset` | Unix timestamp when the current minute window resets |

If you are rate limited, these additional headers are included:

| Header | Description |
|--------|-------------|
| `Retry-After` | Seconds to wait before retrying |

---

## Handling 429 Responses

When you exceed a limit, the response is:

```json
{
  "error": "rate limit exceeded"
}
```

With headers:
```
HTTP/1.1 429 Too Many Requests
Retry-After: 45
X-RateLimit-Limit-Minute: 1000
X-RateLimit-Remaining-Minute: 0
X-RateLimit-Reset: 1755622800
```

### Best Practices

1. **Read `Retry-After`** — Wait at least this many seconds before retrying
2. **Exponential backoff** — If retrying, use exponential backoff with jitter
3. **Monitor `X-RateLimit-Remaining-Minute`** — Slow down proactively as you approach the limit
4. **Cache aggressively** — Cacheable methods (like `eth_chainId`) don't count against your quota

### Example: Exponential Backoff in Python

```python
import time
import random

def call_with_backoff(func, max_retries=5):
    for attempt in range(max_retries):
        response = func()
        if response.status_code != 429:
            return response

        retry_after = int(response.headers.get('Retry-After', 60))
        sleep_time = retry_after * (2 ** attempt) + random.uniform(0, 1)
        time.sleep(min(sleep_time, 300))  # Cap at 5 minutes

    raise Exception("Rate limited after max retries")
```

---

## Rate Limit Scenarios

### Scenario 1: Burst Traffic

Your app sends 20 requests in one second, but your RPS limit is 10:

- Requests 1–10: Accepted (counter = 10)
- Requests 11–20: Rejected with `429`, `Retry-After: 1`

### Scenario 2: Sustained Load

Your app sends 1,200 requests over 2 minutes, but your RPM limit is 1,000:

- Minute 1: 1,000 requests accepted, 200 rejected
- Minute 2: Counter resets, 1,000 requests accepted again

### Scenario 3: Daily Cap

Your daily limit is 100,000. You've sent 99,999 requests today:

- Request 100,000: Accepted
- Request 100,001: Rejected with `429`, `Retry-After` = seconds until midnight

---

## What Happens If Redis Is Down?

BlockMesh **fails closed**. If Redis is unavailable:

- All requests receive `503 Service Unavailable`
- Response body: `{"error":"rate limiter unavailable"}`
- The `blockmesh_gateway_rate_limit_errors_total` Prometheus counter increments

This prevents rate limit bypass during Redis outages.

---

## Checking Your Quotas

Your administrator sets quotas when creating or updating your tenant. Typical defaults:

| Plan | RPS | RPM | Daily |
|------|-----|-----|-------|
| Free | 10 | 100 | 10,000 |
| Basic | 25 | 500 | 50,000 |
| Pro | 50 | 2,000 | 200,000 |
| Enterprise | Custom | Custom | Custom |

Contact your administrator to view or adjust your quotas.

---

## Optimization Tips

1. **Use cacheable methods when possible** — `eth_chainId`, `eth_blockNumber`, `eth_getBalance`, `eth_gasPrice`
2. **Batch requests** — Some JSON-RPC methods support batch arrays (send multiple requests in one HTTP call)
3. **Cache locally** — Store `eth_chainId` for the session; it never changes for a given network
4. **Use `latest` tag wisely** — Balance between freshness and cache hit rate

---

## Related Documents

- [Quick Start](quickstart.md) — Making API calls
- [Authentication](authentication.md) — API key security
- [Errors](errors.md) — Complete error reference including 429 and 503
