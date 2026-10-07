#!/bin/sh
# Regenerates the README screenshots from a running demo (make demo).
# Requires Google Chrome; set CHROME to its path if it is not the macOS default.
set -eu

CHROME=${CHROME:-"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"}
BASE=${BASE:-http://localhost:8080}
OUT=docs/images
RANGE="from=now-30m&to=now"

shot() { # name path height [light]
  scheme="--force-dark-mode --blink-settings=preferredColorScheme=0"
  [ "${4:-}" = light ] && scheme="--blink-settings=preferredColorScheme=1"
  # shellcheck disable=SC2086
  "$CHROME" --headless=new --disable-gpu --hide-scrollbars $scheme --force-device-scale-factor=2 \
    --window-size=1440,"$3" --virtual-time-budget=10000 --screenshot="$OUT/$1.png" "$BASE$2" >/dev/null 2>&1
  echo "  $OUT/$1.png"
}

trace=$(curl -sf "$BASE/api/v1/traces?$RANGE&errors=true&min_duration_ms=500&limit=20" |
  grep -o '"trace_id":"[0-9a-f]*"[^}]*"span_count":9' | head -1 | cut -d'"' -f4)
[ -n "$trace" ] || { echo "no slow failing trace yet: let the demo run a few minutes" >&2; exit 1; }

echo "Writing screenshots:"
shot services "/services?$RANGE" 620
shot services-light "/services?$RANGE" 620 light
shot service "/services/checkout?$RANGE" 760
shot trace "/traces/$trace" 760
shot logs "/logs?$RANGE" 760
shot metrics "/metrics?$RANGE&metric=http.server.request.duration&agg=p95&by=service.name" 620
shot alerts "/alerts" 620
