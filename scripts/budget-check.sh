#!/bin/sh
# Bundle budget gate: initial JS (app.js) must stay <= 50KB gzipped.
# Fails CI over budget. Editor/dnd/image-upload chunks are split out of
# the initial payload by design (vanilla shell; heavy modules lazy-load).
set -eu
BUDGET_KB="${BUDGET_KB:-50}"
FILE="${1:-web/app.js}"
SIZE=$(gzip -c "$FILE" | wc -c)
KB=$(awk "BEGIN {printf \"%.1f\", $SIZE/1024}")
echo "app.js gzip: ${SIZE} bytes (${KB} KB), budget: ${BUDGET_KB} KB"
if [ "$SIZE" -gt $((BUDGET_KB * 1024)) ]; then
  echo "BUDGET EXCEEDED: initial JS over ${BUDGET_KB}KB gzip" >&2
  exit 1
fi
echo "budget OK"
