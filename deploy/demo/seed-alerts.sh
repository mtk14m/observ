#!/bin/sh
# Seeds the demo with a webhook channel and three alert rules, once.
set -eu
BASE=http://obsrv:8080/api/v1

# Sign in with the demo admin account. The session cookie is sent back
# explicitly: curl's cookie jar ignores single-label hosts such as "obsrv".
SESSION=$(curl -sf -D - -o /dev/null -X POST "$BASE/auth/login" -H 'Content-Type: application/json' \
  -d '{"email":"'"$OBSRV_ADMIN_EMAIL"'","password":"'"$OBSRV_ADMIN_PASSWORD"'"}' |
  sed -n 's/^[Ss]et-[Cc]ookie: \(obsrv_session=[^;]*\).*/\1/p')
[ -n "$SESSION" ] || { echo "could not sign in to obsrv" >&2; exit 1; }
api() { curl -sf -H "Cookie: $SESSION" -H 'Content-Type: application/json' "$@"; }

if api "$BASE/alerts/rules" | grep -q '"name"'; then
  echo "alert rules already exist, nothing to do"
  exit 0
fi

CHANNEL=$(api -X POST "$BASE/alerts/channels" \
  -d '{"name":"demo webhook","type":"webhook","url":"http://webhook:8080/obsrv"}' |
  sed -E 's/.*"id":"([^"]+)".*/\1/')
[ -n "$CHANNEL" ] || { echo "could not create the demo channel" >&2; exit 1; }

rule() { api -X POST "$BASE/alerts/rules" -d "$1" >/dev/null; }

rule '{"name":"Payment refusals","kind":"logs","query":"service:payment level:error","op":">","threshold":3,
  "window_seconds":300,"for_seconds":0,"channels":["'"$CHANNEL"'"],"enabled":true}'
rule '{"name":"Slow checkout (p95)","kind":"metric","metric":"http.server.request.duration","agg":"p95",
  "filters":{"service.name":"checkout"},"op":">","threshold":0.5,"window_seconds":300,"for_seconds":60,
  "channels":["'"$CHANNEL"'"],"enabled":true}'
rule '{"name":"Inventory errors","kind":"logs","query":"service:inventory level:error","group_by":["service.name"],
  "op":">","threshold":0,"window_seconds":300,"for_seconds":0,"channels":["'"$CHANNEL"'"],"enabled":true}'

echo "seeded 3 alert rules sending to the demo webhook"
