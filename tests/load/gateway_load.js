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
const API_KEY = __ENV.TEST_API_KEY || '';
const SUMMARY_PATH = __ENV.SUMMARY_PATH || '/tmp/chainmesh-load/load-summary.md';

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
    'has result': (r) => {
      try { return JSON.parse(r.body).result !== undefined; }
      catch (e) { return false; }
    },
    'cache header present': (r) => r.headers['X-Cache'] !== undefined,
  });

  sleep(1);
}

// Fully self-contained summary (no external imports).
export function handleSummary(data) {
  const dur = data.metrics.http_req_duration;
  const failed = data.metrics.http_req_failed;
  const reqs = data.metrics.http_reqs;

  const p95 = (dur && dur.values['p(95)'] !== undefined) ? dur.values['p(95)'] : 0;
  const avg = (dur && dur.values['avg'] !== undefined) ? dur.values['avg'] : 0;
  const failRate = (failed && failed.values['rate'] !== undefined) ? failed.values['rate'] * 100 : 0;
  const total = (reqs && reqs.values['count'] !== undefined) ? reqs.values['count'] : 0;

  const p95Ok = p95 < 250;
  const errOk = failRate < 1;
  const passed = p95Ok && errOk;
  const verdict = passed ? '✅' : '❌';

  const markdown = [
    `## ${verdict} ChainMesh Load Test Results`,
    '',
    '| Metric | Value | Threshold | Status |',
    '|---|---|---|---|',
    `| p(95) Latency | ${p95.toFixed(2)} ms | < 250 ms | ${p95Ok ? '✅ Pass' : '❌ Fail'} |`,
    `| Avg Latency | ${avg.toFixed(2)} ms | — | — |`,
    `| Error Rate | ${failRate.toFixed(2)}% | < 1% | ${errOk ? '✅ Pass' : '❌ Fail'} |`,
    '',
    `*Total requests: ${total}*`,
    '',
  ].join('\n');

  const stdout = [
    '',
    `=== LOAD TEST SUMMARY ${verdict} ===`,
    `p(95):   ${p95.toFixed(2)} ms  (threshold < 250 ms)  ${p95Ok ? 'PASS' : 'FAIL'}`,
    `avg:     ${avg.toFixed(2)} ms`,
    `errors:  ${failRate.toFixed(2)}%  (threshold < 1%)    ${errOk ? 'PASS' : 'FAIL'}`,
    `total:   ${total} requests`,
    '',
  ].join('\n');

  const out = {};
  out[SUMMARY_PATH] = markdown;   // markdown file → scp'd back → job summary
  out['stdout'] = stdout;          // plain text → shows in the step log
  return out;
}