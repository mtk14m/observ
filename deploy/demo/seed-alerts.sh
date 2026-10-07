#!/bin/sh
# Seeds the demo with a webhook channel and three alert rules, once.
set -eu
API=http://obsrv:8080/api/v1/alerts

if curl -sf "$API/rules" | grep -q '"name"'; then
  echo "alert rules already exist, nothing to do"
  exit 0
fi

CHANNEL=$(curl -sf -X POST "$API/channels" -H 'Content-Type: application/json' \
  -d '{"name":"demo webhook","type":"webhook","url":"http://webhook:8080/obsrv"}' |
  sed -E 's/.*"id":"([^"]+)".*/\1/')

rule() {
  curl -sf -X POST "$API/rules" -H 'Content-Type: application/json' -d "$1" >/dev/null
}

rule '{"name":"Payment refusals","kind":"logs","query":"service:payment level:error","op":">","threshold":3,
  "window_seconds":300,"for_seconds":0,"channels":["'"$CHANNEL"'"],"enabled":true}'
rule '{"name":"Slow checkout (p95)","kind":"metric","metric":"http.server.request.duration","agg":"p95",
  "filters":{"service.name":"checkout"},"op":">","threshold":0.5,"window_seconds":300,"for_seconds":60,
  "channels":["'"$CHANNEL"'"],"enabled":true}'
rule '{"name":"Inventory errors","kind":"logs","query":"service:inventory level:error","group_by":["service.name"],
  "op":">","threshold":0,"window_seconds":300,"for_seconds":0,"channels":["'"$CHANNEL"'"],"enabled":true}'

echo "seeded 3 alert rules sending to the demo webhook"
