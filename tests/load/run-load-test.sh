#!/bin/sh
# GITHUB_STEP_SUMMARY is automatically inherited from the Actions runner env
k6 run --env GATEWAY_URL="$GATEWAY_URL" --env TEST_API_KEY="$TEST_API_KEY" tests/load/gateway_load.js