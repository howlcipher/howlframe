#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/../.."
export GOCACHE="${GOCACHE:-/tmp/hf-token-go-cache}"
go build -o /tmp/howlframe .
go build -o /tmp/hfexp ./experiments/falsifiable-v0/cmd/hfexp
go build -o /tmp/tokverify ./experiments/token-efficiency-v0/cmd/tokverify
# Count failures too, then propagate the verifier's nonzero status.
status=0
/tmp/tokverify || status=$?
"${TOKBENCH_PYTHON:-python3}" experiments/token-efficiency-v0/count_tokens.py
exit "$status"
