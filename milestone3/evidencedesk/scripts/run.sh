#!/usr/bin/env bash
set -euo pipefail
app=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
compiler=${HOWLFRAME_BIN:-$app/build/howlframe}
export EVIDENCEDESK_DATA=${EVIDENCEDESK_DATA:-$app/build/state.json}
export EVIDENCEDESK_STATIC=${EVIDENCEDESK_STATIC:-$app/static}
export EVIDENCEDESK_PORT=${EVIDENCEDESK_PORT:-8093}
EVIDENCEDESK_DATA=$(realpath -m -- "$EVIDENCEDESK_DATA")
EVIDENCEDESK_STATIC=$(realpath -- "$EVIDENCEDESK_STATIC")
export EVIDENCEDESK_DATA EVIDENCEDESK_STATIC
exec "$compiler" run --listen-host 127.0.0.1 --allow-caps "network,environment,database,filesystem:read=$EVIDENCEDESK_DATA,filesystem:write=$EVIDENCEDESK_DATA,filesystem:read=$EVIDENCEDESK_STATIC" --max-instructions 10000000 --max-request-bytes 300000 "$app/build/backend.hfbc"
