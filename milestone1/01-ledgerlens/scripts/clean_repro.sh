#!/usr/bin/env bash
# Clean-environment reproduction for ledgerlens (D24-D).
# Builds HowlFrame and the app from the COMMITTED tree only (git archive HEAD into a
# fresh temp dir), then runs the frozen acceptance suite. Needs: git, go, python3.
set -euo pipefail
repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
git -C "$repo" archive HEAD | tar -x -C "$work"
app="$work/milestone1/01-ledgerlens"
(cd "$work" && go build -o "$work/howlframe" .)
mkdir -p "$work/out"
"$work/howlframe" build "$app/src/ledgerlens.howl" -o "$work/out/ledgerlens.hfbc"
export HF_BIN="$work/howlframe" APP_ARTIFACT="$work/out/ledgerlens.hfbc"
python3 "$app/tests/acceptance/run_acceptance.py" "$app/tests/acceptance/launcher.example.sh" "$@"
