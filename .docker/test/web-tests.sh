#!/bin/sh
set -e

cd /app
npm ci

mkdir -p /app/test-results/web

npx vitest run \
  --reporter=verbose \
  --reporter=json \
  --outputFile=/app/test-results/web/vitest-results.json \
  --coverage \
  --coverage.reporter=text \
  --coverage.reporter=html \
  --coverage.reportsDirectory=/app/test-results/web/coverage \
  2>&1 | tee /app/test-results/web/vitest.log