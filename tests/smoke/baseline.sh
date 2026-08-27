#!/bin/bash
# tests/smoke/baseline.sh
set -e

SAMPLES=20
echo "Collecting ${SAMPLES} latency samples..."

TOTAL=0
MIN=999999
MAX=0

for i in $(seq 1 $SAMPLES); do
  START=$(date +%s%N)
  curl -sf -X POST "${GATEWAY_URL}" \
    -H "Authorization: Bearer ${TEST_API_KEY}" \
    -H "Content-Type: application/json" \
    -d '{"jsonrpc":"2.0","method":"eth_chainId","params":[],"id":1}' > /dev/null
  END=$(date +%s%N)
  
  LATENCY=$(( (END - START) / 1000000 ))  # ms
  TOTAL=$((TOTAL + LATENCY))
  [ $LATENCY -lt $MIN ] && MIN=$LATENCY
  [ $LATENCY -gt $MAX ] && MAX=$LATENCY
  echo "  Sample $i: ${LATENCY}ms"
done

AVG=$((TOTAL / SAMPLES))

echo ""
echo "=== Baseline Results ==="
echo "Average: ${AVG}ms"
echo "Min:     ${MIN}ms"
echo "Max:     ${MAX}ms"
echo ""
echo "Alert thresholds:"
echo "  AVG > 100ms  → investigate"
echo "  AVG > 300ms  → page on-call"

# Fail the script if average latency is critically high
if [ $AVG -gt 300 ]; then
  echo "✗ CRITICAL: Average latency exceeds 300ms!"
  exit 1
fi