#!/usr/bin/env bash
# Post-records rerun: same frozen specs, same Python/JS/Go references and the
# same verifier/tokenizer as run.sh, HowlFrame sources from postrec/. The
# baseline (solutions/howl, results/) is not touched.
set -euo pipefail
cd "$(dirname "$0")/../.."
export GOCACHE="${GOCACHE:-/tmp/hf-token-go-cache}"
go build -o /tmp/howlframe .
go build -o /tmp/heldoutv3verify ./experiments/token-efficiency-heldout-v3/cmd/heldoutv3verify
B=experiments/token-efficiency-heldout-v3
status=0
/tmp/heldoutv3verify -howlframe /tmp/howlframe -howl-dir "$B/postrec/solutions/howl" -results "$B/postrec/results" || status=$?
if [ "$status" -eq 0 ]; then
  "${TOKBENCH_PYTHON:-python3}" "$B/count_tokens.py" --howl-dir postrec/solutions/howl --results postrec/results
  "${TOKBENCH_PYTHON:-python3}" "$B/compare_postrec.py"
fi
exit "$status"
