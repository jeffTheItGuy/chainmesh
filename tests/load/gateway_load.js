// tests/load/gateway_load.js
import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  stages: [
    { duration: '2m', target: 100 },   // Ramp up
    { duration: '5m', target: 100 },   // Steady state
    { duration: '2m', target: 200 },   // Stress
    { duration: '2m', target: 0 },     // Ramp down
  ],
  thresholds: {
    http_req_duration: ['p(95)<250'],   // 95th percentile < 250ms
    http_req_failed: ['rate<0.01'],     // Error rate < 1%
  },
};

const GATEWAY_URL = __ENV.GATEWAY_URL || 'http://localhost:8080/v1/';
const API_KEY = __ENV.TEST_API_KEY;

export default function () {
  const payload = JSON.stringify({
    jsonrpc: '2.0',
    method: 'eth_chainId',
    params: [],
    id: 1,
  });

  const res = http.post(GATEWAY_URL, payload, {
    headers: {
      'Authorization': `Bearer ${API_KEY}`,
      'Content-Type': 'application/json',
    },
  });

  check(res, {
    'status is 200': (r) => r.status === 200,
    'has result': (r) => JSON.parse(r.body).result !== undefined,
    'cache header present': (r) => r.headers['X-Cache'] !== undefined,
  });

  sleep(1);
}