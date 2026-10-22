#!/usr/bin/env bash
# Runs the accepted application suites against the current tree: Milestone 1 (LedgerLens,
# HealthWatch, TaskFlow) and Milestone 2 (TicketDesk), with their retained regressions.
# Used by CI (.github/workflows/ci.yml, job "acceptance") and runnable locally:
#
#   scripts/ci_acceptance.sh            # builds howlframe itself
#   HF_BIN=/path/to/howlframe scripts/ci_acceptance.sh
#
# Requirements: Go, Python 3 (standard library only), bash. Loopback networking only.
# Every suite must exit 0; the first failure stops the script with that suite's status.
# Not covered here on purpose: Milestone 3 (needs real Chromium, Playwright and a sibling HowlBoard
# checkout; see milestone3/evidencedesk/README.md for the manual verification steps).
set -euo pipefail

repo=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
cd "$repo"
work=$(mktemp -d)
trap 'rm -rf -- "$work"' EXIT

if [[ -z ${HF_BIN:-} ]]; then
  HF_BIN="$work/howlframe"
  go build -o "$HF_BIN" howlframe.go
fi
export HF_BIN
export HOWL_FORBID_LOCAL_INFERENCE=1

section() { printf '\n==== %s ====\n' "$*"; }

section "Milestone 1: frozen acceptance suites"
for entry in 01-ledgerlens:ledgerlens 02-healthwatch:healthwatch 03-taskflow:taskflow; do
  dir=${entry%%:*}
  name=${entry##*:}
  "$HF_BIN" build "milestone1/$dir/src/$name.howl" -o "$work/$name.hfbc"
  section "$name"
  APP_ARTIFACT="$work/$name.hfbc" python3 "milestone1/$dir/tests/acceptance/run_acceptance.py" \
    "milestone1/$dir/tests/acceptance/launcher.example.sh"
done

section "Milestone 1: retained regressions"
M1_ARTIFACT_DIR="$work" python3 milestone1/tests/regression/fresh_boundaries.py
APP_ARTIFACT="$work/healthwatch.hfbc" python3 milestone1/rebuild/02-healthwatch/tests/supplemental.py
APP_ARTIFACT="$work/ledgerlens.hfbc" bash milestone1/01-ledgerlens/tests/regression/rules_lowest_line.sh \
  milestone1/01-ledgerlens/tests/acceptance/launcher.example.sh

section "Milestone 2: TicketDesk"
"$HF_BIN" build milestone2/ticketdesk/src/ticketdesk.howl -o "$work/ticketdesk.hfbc"
export APP_ARTIFACT="$work/ticketdesk.hfbc"
launcher=milestone2/ticketdesk/tests/acceptance/launcher.example.sh
section "frozen acceptance (101)"
python3 milestone2/ticketdesk/tests/acceptance/run_acceptance.py "$launcher"
section "developer-experience checks (13)"
python3 milestone2/ticketdesk/dx/check_dx.py
section "reassessment / adversarial"
python3 milestone2/rebuild/ticketdesk/tests/run_reassessment.py "$launcher"
section "historical review regressions"
python3 milestone2/ticketdesk/tests/regression/review_round1.py "$launcher"
section "Assurance m10 (strict expected failures allowed only as recorded)"
python3 milestone2/ticketdesk/tests/regression/assurance_m10.py "$launcher"
section "restart at default ceilings (1,000 tickets, 100 tokens), SIGKILL, invalid state"
python3 milestone2/ticketdesk/tests/regression/restart_default_budget.py "$launcher"

section "all accepted-application suites passed"
