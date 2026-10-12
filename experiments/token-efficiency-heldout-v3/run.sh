#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/../.."
export GOCACHE="${GOCACHE:-/tmp/hf-token-go-cache}"
go build -o /tmp/howlframe .
go build -o /tmp/heldoutv3verify ./experiments/token-efficiency-heldout-v3/cmd/heldoutv3verify
status=0
/tmp/heldoutv3verify -howlframe /tmp/howlframe || status=$?
if [ "$status" -eq 0 ]; then
  "${TOKBENCH_PYTHON:-python3}" experiments/token-efficiency-heldout-v3/count_tokens.py
fi
exit "$status"
