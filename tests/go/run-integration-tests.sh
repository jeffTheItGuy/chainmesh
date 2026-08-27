#!/bin/sh

mkdir -p /app/test-results

apk add --no-cache git
go mod tidy

echo "=== Running integration tests ===" | tee /app/test-results/integration-full.log

# Run tests, capture everything
go test ./... -tags=integration -v >> /app/test-results/integration-full.log 2>&1
EXIT_CODE=$?

# Extract only the lines that matter for a quick success/failure check.
# Note: Go's -v output uses "--- PASS:" / "--- FAIL:" (colon, not whitespace)
# right after PASS/FAIL, so the pattern must match that literally or these
# per-test lines get silently dropped from the summary.
grep -E "^(=== RUN|--- PASS:|--- FAIL:|PASS|FAIL|ok)" /app/test-results/integration-full.log > /app/test-results/integration-summary.log || true

# Also print the summary to stdout so it shows up in `docker compose` output
echo ""
echo "=== Integration Test Summary ==="
cat /app/test-results/integration-summary.log
echo "================================"
echo "Full log: test-results/integration-full.log"
echo "Summary:  test-results/integration-summary.log"

exit $EXIT_CODE