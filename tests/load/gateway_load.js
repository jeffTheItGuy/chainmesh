export function handleSummary(data) {
  const reqDuration = data.metrics.http_req_duration;
  const reqFailed = data.metrics.http_req_failed;

  const p95 = reqDuration ? reqDuration.values['p(95)'].toFixed(2) : 'N/A';
  const avg = reqDuration ? reqDuration.values['avg'].toFixed(2) : 'N/A';
  const failRate = reqFailed ? (reqFailed.values['rate'] * 100).toFixed(2) : 'N/A';

  const p95Pass = reqDuration && reqDuration.thresholds['p(95)<250'].ok;
  const errPass = reqFailed && reqFailed.thresholds['rate<0.01'].ok;
  const verdict = (p95Pass && errPass) ? "✅" : "❌";

  const mdSummary = `## ${verdict} ChainMesh Load Test Results\n\n` +
    `| Metric | Value | Threshold | Status |\n` +
    `|---|---|---|---|\n` +
    `| p(95) Latency | ${p95}ms | < 250ms | ${p95Pass ? '✅ Pass' : '❌ Fail'} |\n` +
    `| Avg Latency | ${avg}ms | - | - |\n` +
    `| Error Rate | ${failRate}% | < 1% | ${errPass ? '✅ Pass' : '❌ Fail'} |\n\n` +
    `*Total Requests: ${data.metrics.http_reqs.values.count}*\n`;

  const outputs = {
    'stdout': textSummary(data, { indent: ' ', enableColors: true }),
  };

  // Write directly to the GitHub job summary file if running in Actions
  const summaryPath = __ENV.GITHUB_STEP_SUMMARY;
  if (summaryPath) {
    outputs[summaryPath] = mdSummary;
  }

  return outputs;
}