# Load Test Results

## Smoke Test
- VUs: 1
- Duration: 10s
- Status: 200 OK
- p99 latency: ~120ms (uncached), ~5ms (cached)

## Target
- 4,000 req/s sustained
- p99 < 50ms with Redis cache
- p99 < 800ms without cache
