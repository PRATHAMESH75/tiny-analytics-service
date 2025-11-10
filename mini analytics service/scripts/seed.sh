#!/usr/bin/env bash

set -euo pipefail

INGEST_URL=${INGEST_URL:-http://localhost:8080/v1/collect}
INGEST_API_KEY=${INGEST_API_KEY:-dev-site-1-key}
EVENT_COUNT=${EVENT_COUNT:-20}

echo "Seeding ${EVENT_COUNT} events to ${INGEST_URL}"

for i in $(seq 1 "${EVENT_COUNT}"); do
  site=$(( (i % 3) + 1 ))
  user=$(( (i % 5) + 1 ))
  now_ms=$(( $(date +%s) * 1000 ))
  ts=$(( now_ms + i ))
  payload=$(cat <<JSON
{
  "site_id": "site-${site}",
  "event_name": "pageview",
  "url": "https://example.com/page-${i}",
  "referrer": "https://referrer.example.com",
  "user_id": "user-${user}",
  "session_id": "session-${user}",
  "ts": ${ts},
  "props": {
    "sample": ${i},
    "feature": "seed-script"
  }
}
JSON
)
  curl -sS -X POST "${INGEST_URL}" \
    -H "Content-Type: application/json" \
    -H "X-TA-API-Key: ${INGEST_API_KEY}" \
    -d "${payload}" >/dev/null
done

echo "Seeding complete."
