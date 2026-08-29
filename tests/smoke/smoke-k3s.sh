#!/bin/bash
# tests/smoke/smoke-k3s.sh
# Runs ON the k3s node, hitting ClusterIP services directly.
# No Cloudflare, no public DNS, no external network.

export KUBECONFIG="${KUBECONFIG:-/etc/rancher/k3s/k3s.yaml}"
NAMESPACE="${NAMESPACE:-chainmesh}"
RESULTS_FILE="${RESULTS_FILE:-/tmp/chainmesh-smoke/results.md}"

# Ensure the directory exists
mkdir -p "$(dirname "$RESULTS_FILE")"

PASS=0
FAIL=0
ROWS=""

record() {
  local name="$1" status="$2" detail="$3"
  if [ "$status" = "pass" ]; then
    PASS=$((PASS+1)); ROWS="${ROWS}| ${name} | ✅ Pass | ${detail} |\n"
  elif [ "$status" = "fail" ]; then
    FAIL=$((FAIL+1)); ROWS="${ROWS}| ${name} | ❌ Fail | ${detail} |\n"
  else
    ROWS="${ROWS}| ${name} | ⏭️ Skip | ${detail} |\n"
  fi
}

# --- Resolve internal ClusterIP addresses (no DNS/Cloudflare involved) ---
GATEWAY_SVC=$(kubectl get svc -n "$NAMESPACE" -o name 2>/dev/null | grep -i gateway | head -1 | sed 's|service/||')
WEB_SVC=$(kubectl get svc -n "$NAMESPACE" -o name 2>/dev/null | grep -i web | head -1 | sed 's|service/||')

if [ -z "$GATEWAY_SVC" ]; then
  echo "❌ Could not find a gateway service in namespace '$NAMESPACE'" >&2
  exit 2
fi

GATEWAY_IP=$(kubectl get svc "$GATEWAY_SVC" -n "$NAMESPACE" -o jsonpath='{.spec.clusterIP}')
GATEWAY_PORT=$(kubectl get svc "$GATEWAY_SVC" -n "$NAMESPACE" -o jsonpath='{.spec.ports[0].port}')
GATEWAY_URL="http://${GATEWAY_IP}:${GATEWAY_PORT}/v1/"

WEB_URL=""
if [ -n "$WEB_SVC" ]; then
  WEB_IP=$(kubectl get svc "$WEB_SVC" -n "$NAMESPACE" -o jsonpath='{.spec.clusterIP}')
  WEB_PORT=$(kubectl get svc "$WEB_SVC" -n "$NAMESPACE" -o jsonpath='{.spec.ports[0].port}')
  WEB_URL="http://${WEB_IP}:${WEB_PORT}/"
fi

echo "Gateway: $GATEWAY_URL"
echo "Web:     ${WEB_URL:-not found}"
echo ""

# --- 1. Gateway reachable (any HTTP response = service is up) ---
CODE=$(curl -s -o /dev/null -w "%{http_code}" --max-time 10 -X POST "$GATEWAY_URL" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"eth_chainId","params":[],"id":1}')

if [ "$CODE" != "000" ]; then
  record "Gateway reachable" "pass" "HTTP $CODE"
else
  record "Gateway reachable" "fail" "no response (000)"
fi

if [ -n "$TEST_API_KEY" ]; then
  # --- 2. Authenticated RPC returns a result ---
  RESP=$(curl -s --max-time 10 -X POST "$GATEWAY_URL" \
    -H "Authorization: Bearer ${TEST_API_KEY}" \
    -H "Content-Type: application/json" \
    -d '{"jsonrpc":"2.0","method":"eth_chainId","params":[],"id":1}')
  
  if echo "$RESP" | grep -q '"result"'; then
    record "Authenticated RPC" "pass" "returned result"
  else
    record "Authenticated RPC" "fail" "no result in response"
  fi

  # --- 3. Cache MISS on first call ---
  H1=$(curl -s --max-time 10 -X POST "$GATEWAY_URL" \
    -H "Authorization: Bearer ${TEST_API_KEY}" \
    -H "Content-Type: application/json" \
    -d '{"jsonrpc":"2.0","method":"eth_chainId","params":[],"id":1}' -D - -o /dev/null | grep -i "X-Cache:")
  
  if echo "$H1" | grep -qi "MISS"; then
    record "Cache MISS (1st call)" "pass" "$(echo "$H1" | tr -d '\r')"
  else
    record "Cache MISS (1st call)" "fail" "${H1:-no X-Cache header}"
  fi

  # --- 4. Cache HIT on second call ---
  H2=$(curl -s --max-time 10 -X POST "$GATEWAY_URL" \
    -H "Authorization: Bearer ${TEST_API_KEY}" \
    -H "Content-Type: application/json" \
    -d '{"jsonrpc":"2.0","method":"eth_chainId","params":[],"id":1}' -D - -o /dev/null | grep -i "X-Cache:")
  
  if echo "$H2" | grep -qi "HIT"; then
    record "Cache HIT (2nd call)" "pass" "$(echo "$H2" | tr -d '\r')"
  else
    record "Cache HIT (2nd call)" "fail" "${H2:-no X-Cache header}"
  fi

  # --- 5. Rate limit headers present ---
  RH=$(curl -s --max-time 10 -X POST "$GATEWAY_URL" \
    -H "Authorization: Bearer ${TEST_API_KEY}" \
    -H "Content-Type: application/json" \
    -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' -D - -o /dev/null)
  
  if echo "$RH" | grep -qi "X-RateLimit-Remaining"; then
    record "Rate limit headers" "pass" "present"
  else
    record "Rate limit headers" "fail" "missing"
  fi
else
  record "Authenticated RPC" "skip" "TEST_API_KEY not set"
  record "Cache MISS (1st call)" "skip" "TEST_API_KEY not set"
  record "Cache HIT (2nd call)" "skip" "TEST_API_KEY not set"
  record "Rate limit headers" "skip" "TEST_API_KEY not set"
fi

# --- 6. Web dashboard reachable (internal ClusterIP) ---
if [ -n "$WEB_URL" ]; then
  WCODE=$(curl -s -o /dev/null -w "%{http_code}" --max-time 10 "$WEB_URL")
  if [ "$WCODE" = "200" ]; then
    record "Web dashboard" "pass" "HTTP $WCODE"
  else
    record "Web dashboard" "fail" "HTTP $WCODE"
  fi
else
  record "Web dashboard" "skip" "web service not found"
fi

# --- Render markdown results ---
TOTAL=$((PASS+FAIL))
if [ "$FAIL" -eq 0 ]; then VERDICT="✅"; else VERDICT="❌"; fi

{
  echo "## ${VERDICT} ChainMesh Smoke Test — in-cluster"
  echo ""
  echo "**Namespace:** \`$NAMESPACE\`  ·  **Gateway svc:** \`$GATEWAY_SVC\`  ·  **Passed:** $PASS/$TOTAL"
  echo ""
  echo "| Check | Status | Detail |"
  echo "|-------|--------|--------|"
  printf "%b" "$ROWS"
} > "$RESULTS_FILE"

# Print to standard workflow logs
cat "$RESULTS_FILE"

# ✨ Append to GitHub Actions Job Summary UI
if [ -n "$GITHUB_STEP_SUMMARY" ]; then
  cat "$RESULTS_FILE" >> "$GITHUB_STEP_SUMMARY"
fi

# Exit non-zero if any check failed (drives the workflow's red/green status)
[ "$FAIL" -eq 0 ]