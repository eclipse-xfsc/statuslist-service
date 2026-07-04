#!/usr/bin/env bash
set -euo pipefail

NATS_SERVER="${NATS_SERVER:-nats://localhost:4222}"
TOPIC="${TOPIC:-status.data.create}"

TENANT_ID="${TENANT_ID:-tenant-a}"
REQUEST_ID="${REQUEST_ID:-req-$(date +%s)}"
ORIGIN="${ORIGIN:-http://localhost:8080}"
KEY="${KEY:-test-key}"
DID="${DID:-did:web:localhost}"
NAMESPACE="${NAMESPACE:-default}"
GROUP="${GROUP:-default}"
PURPOSE="${PURPOSE:-revocation}"
EXPIRATION_DATE="${EXPIRATION_DATE:-2027-07-04T21:00:00Z}"
COUNT="${COUNT:-1}"

TYPE="${TYPE:-BitstringStatusListCredential}"

case "$TYPE" in
  BitstringStatusListCredential|BitstringStatusList|StatusList2021|StatusList2021Credential)
    ;;
  *)
    echo "Unsupported TYPE: $TYPE" >&2
    echo "Allowed: BitstringStatusListCredential, BitstringStatusList, StatusList2021, StatusList2021Credential" >&2
    exit 1
    ;;
esac

for i in $(seq 1 "$COUNT"); do
  REQ_ID="${REQUEST_ID}-${i}"

  RESPONSE="$(nats request "$TOPIC" "{
    \"specversion\": \"1.0\",
    \"id\": \"$REQ_ID\",
    \"source\": \"mock-client\",
    \"type\": \"$TOPIC\",
    \"datacontenttype\": \"application/json\",
    \"data\": {
      \"tenant_id\": \"$TENANT_ID\",
      \"request_id\": \"$REQ_ID\",
      \"origin\": \"$ORIGIN\",
      \"key\": \"$KEY\",
      \"did\": \"$DID\",
      \"namespace\": \"$NAMESPACE\",
      \"group\": \"$GROUP\",
      \"type\": \"$TYPE\",
      \"purpose\": \"$PURPOSE\",
      \"expirationDate\": \"$EXPIRATION_DATE\"
    }
  }" --server "$NATS_SERVER")"

  echo "$RESPONSE"

  if command -v jq >/dev/null 2>&1; then
    echo "$RESPONSE" | jq -r '.data.statusUrl // .data.statusURL // .statusUrl // .statusURL // empty'
  fi
done