#!/usr/bin/env bash
# Clean-environment reproduction for Milestone 2 (gate G).
# Builds HowlFrame and the ticketdesk server from the COMMITTED tree only (git archive HEAD into a fresh
# temp dir), then runs the frozen acceptance suite (101 tests) and the developer-experience checker.
# Needs only: git, go, python3 (standard library). Nothing else from the Howl ecosystem.
set -euo pipefail
repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
git -C "$repo" archive HEAD | tar -x -C "$work"
app="$work/milestone2/ticketdesk"
(cd "$work" && go build -o "$work/howlframe" .)
mkdir -p "$work/out"
"$work/howlframe" build "$app/src/ticketdesk.howl" -o "$work/out/ticketdesk.hfbc"
export HF_BIN="$work/howlframe" APP_ARTIFACT="$work/out/ticketdesk.hfbc"
echo "== acceptance"
python3 "$app/tests/acceptance/run_acceptance.py" "$app/tests/acceptance/launcher.example.sh" "$@" | tail -4
echo "== developer-experience checker"
python3 "$app/dx/check_dx.py" | tail -2
