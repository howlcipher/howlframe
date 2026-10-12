#!/usr/bin/env bash
# Post-control-flow rerun: same specs, same Python/JS/Go references and the
# same verifier/tokenizer as run.sh, HowlFrame sources from postcf/. The
# frozen baseline (solutions/howl, results/) is not touched.
set -euo pipefail
cd "$(dirname "$0")/../.."
export GOCACHE="${GOCACHE:-/tmp/hf-token-go-cache}"
go build -o /tmp/howlframe .
go build -o /tmp/heldoutv2verify ./experiments/token-efficiency-heldout-v2/cmd/heldoutv2verify
B=experiments/token-efficiency-heldout-v2
status=0
/tmp/heldoutv2verify -howlframe /tmp/howlframe -howl-dir "$B/postcf/solutions/howl" -results "$B/postcf/results" || status=$?
if [ "$status" -eq 0 ]; then
  "${TOKBENCH_PYTHON:-python3}" "$B/count_tokens.py" --howl-dir postcf/solutions/howl --results postcf/results
  "${TOKBENCH_PYTHON:-python3}" "$B/compare_postcf.py"
fi
exit "$status"
