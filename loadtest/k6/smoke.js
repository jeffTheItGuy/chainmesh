import http from 'k6/http';
import { check } from 'k6';

export const options = {
  vus: 1,
  duration: '10s',
};

export default function () {
  const res = http.post('http://localhost:8080/v1/', JSON.stringify({
    jsonrpc: '2.0',
    method: 'eth_blockNumber',
    params: [],
    id: 1
  }), {
    headers: {
      'Content-Type': 'application/json',
      'Authorization': 'Bearer demo-key'
    }
  });

  check(res, {
    'status is 200': (r) => r.status === 200,
    'response time < 500ms': (r) => r.timings.duration < 500,
  });
}
