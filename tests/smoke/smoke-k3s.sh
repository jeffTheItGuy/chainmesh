#!/usr/bin/env bash
# tests/smoke/smoke-k3s.sh
# Runs ON the k3s node, hitting ClusterIP services directly.
# No Cloudflare, no public DNS, no external network.
#
# Defensive behaviors:
#   - Sanitizes TEST_API_KEY (strips hidden CR/LF/spaces from CI secrets)
#   - Pre-flights kubectl + namespace before running checks
#   - Uses a guaranteed-cold cache key (eth_getBalance + random address)
#     so the MISS/HIT checks don't false-fail on a warm cache
#   - Prints the actual gateway response when auth fails

set -u

export KUBECONFIG="${KUBECONFIG:-/etc/rancher/k3s/k3s.yaml}"
NAMESPACE="${NAMESPACE:-chainmesh}"
RESULTS_FILE="${RESULTS_FILE:-/tmp/chainmesh-smoke/results.md}"

# --- Sanitize the API key (kills hidden newlines/spaces from GitHub secrets) ---
TEST_API_KEY="${TEST_API_KEY:-}"
TEST_API_KEY="$(printf '%s' "$TEST_API_KEY" | tr -d '\r\n' | sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//')"

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

# --- Pre-flight: kubectl + namespace must be usable ---
if ! command -v kubectl >/dev/null 2>&1; then
  echo "❌ kubectl not found on this node" >&2
  exit 2
fi
if ! kubectl get namespace "$NAMESPACE" >/dev/null 2>&1; then
  echo "❌ Namespace '$NAMESPACE' not found (check KUBECONFIG)" >&2
  exit 2
fi
mkdir -p "$(dirname "$RESULTS_FILE")"

# --- Resolve internal ClusterIP addresses (no DNS/Cloudflare involved) ---
GATEWAY_SVC="$(kubectl get svc -n "$NAMESPACE" -o name 2>/dev/null | grep -i gateway | head -1 | sed 's|service/||')"
WEB_SVC="$(kubectl get svc -n "$NAMESPACE" -o name 2>/dev/null | grep -i web | head -1 | sed 's|service/||')"

if [ -z "$GATEWAY_SVC" ]; then
  echo "❌ Could not find a gateway service in namespace '$NAMESPACE'" >&2
  exit 2
fi

GATEWAY_IP="$(kubectl get svc "$GATEWAY_SVC" -n "$NAMESPACE" -o jsonpath='{.spec.clusterIP}')"
GATEWAY_PORT="$(kubectl get svc "$GATEWAY_SVC" -n "$NAMESPACE" -o jsonpath='{.spec.ports[0].port}')"
GATEWAY_URL="http://${GATEWAY_IP}:${GATEWAY_PORT}/v1/"

WEB_URL=""
if [ -n "$WEB_SVC" ]; then
  WEB_IP="$(kubectl get svc "$WEB_SVC" -n "$NAMESPACE" -o jsonpath='{.spec.clusterIP}')"
  WEB_PORT="$(kubectl get svc "$WEB_SVC" -n "$NAMESPACE" -o jsonpath='{.spec.ports[0].port}')"
  WEB_URL="http://${WEB_IP}:${WEB_PORT}/"
fi

echo "Gateway: $GATEWAY_URL"
echo "Web:     ${WEB_URL:-not found}"
echo ""

# --- 1. Gateway reachable (any HTTP response = service is up) ---
CODE="$(curl -s -o /dev/null -w '%{http_code}' --max-time 10 -X POST "$GATEWAY_URL" \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","method":"eth_chainId","params":[],"id":1}' || echo 000)"
if [ "$CODE" != "000" ]; then
  record "Gateway reachable" pass "HTTP $CODE"
else
  record "Gateway reachable" fail "no response (000)"
fi

if [ -n "$TEST_API_KEY" ]; then
  # --- 2. Authenticated RPC returns a result ---
  RESP="$(curl -s --max-time 10 -X POST "$GATEWAY_URL" \
    -H "Authorization: Bearer ${TEST_API_KEY}" \
    -H 'Content-Type: application/json' \
    -d '{"jsonrpc":"2.0","method":"eth_chainId","params":[],"id":1}')"
  if printf '%s' "$RESP" | grep -q '"result"'; then
    record "Authenticated RPC" pass "returned result"
  else
    SHORT="$(printf '%s' "$RESP" | head -c 120 | tr -d '\n\r')"
    record "Authenticated RPC" fail "no result. resp: ${SHORT:-empty}"
  fi

  # --- 3 & 4. Cache MISS then HIT ---
  # Use eth_getBalance with a random address = guaranteed-cold cache key.
  # (eth_chainId would already be warm from check #2 and false-fail the MISS.)
  RAND_ADDR="$(printf '0x%040x' "$(date +%s%N)")"
  BAL_PAYLOAD='{"jsonrpc":"2.0","method":"eth_getBalance","params":["'"$RAND_ADDR"'","latest"],"id":1}'

  H1="$(curl -s --max-time 10 -X POST "$GATEWAY_URL" \
    -H "Authorization: Bearer ${TEST_API_KEY}" \
    -H 'Content-Type: application/json' \
    -d "$BAL_PAYLOAD" -D - -o /dev/null | grep -i '^X-Cache:')"
  if printf '%s' "$H1" | grep -qi 'MISS'; then
    record "Cache MISS (1st call)" pass "$(printf '%s' "$H1" | tr -d '\r')"
  else
    record "Cache MISS (1st call)" fail "${H1:-no X-Cache header}"
  fi

  H2="$(curl -s --max-time 10 -X POST "$GATEWAY_URL" \
    -H "Authorization: Bearer ${TEST_API_KEY}" \
    -H 'Content-Type: application/json' \
    -d "$BAL_PAYLOAD" -D - -o /dev/null | grep -i '^X-Cache:')"
  if printf '%s' "$H2" | grep -qi 'HIT'; then
    record "Cache HIT (2nd call)" pass "$(printf '%s' "$H2" | tr -d '\r')"
  else
    record "Cache HIT (2nd call)" fail "${H2:-no X-Cache header}"
  fi

  # --- 5. Rate limit headers present ---
  RH="$(curl -s --max-time 10 -X POST "$GATEWAY_URL" \
    -H "Authorization: Bearer ${TEST_API_KEY}" \
    -H 'Content-Type: application/json' \
    -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' -D - -o /dev/null)"
  if printf '%s' "$RH" | grep -qi 'X-RateLimit-Remaining'; then
    record "Rate limit headers" pass "present"
  else
    record "Rate limit headers" fail "missing"
  fi
else
  record "Authenticated RPC" skip "TEST_API_KEY not set"
  record "Cache MISS (1st call)" skip "TEST_API_KEY not set"
  record "Cache HIT (2nd call)" skip "TEST_API_KEY not set"
  record "Rate limit headers" skip "TEST_API_KEY not set"
fi

# --- 6. Web dashboard reachable (internal ClusterIP) ---
if [ -n "$WEB_URL" ]; then
  WCODE="$(curl -s -o /dev/null -w '%{http_code}' --max-time 10 "$WEB_URL" || echo 000)"
  if [ "$WCODE" = "200" ]; then
    record "Web dashboard" pass "HTTP $WCODE"
  else
    record "Web dashboard" fail "HTTP $WCODE"
  fi
else
  record "Web dashboard" skip "web service not found"
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

cat "$RESULTS_FILE"

# Exit non-zero if any check failed (drives the workflow's red/green status)
[ "$FAIL" -eq 0 ]