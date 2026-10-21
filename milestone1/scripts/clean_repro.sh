#!/usr/bin/env bash
# Clean-environment reproduction for a Milestone 1 app (D24-D).
# Usage: milestone1/scripts/clean_repro.sh <NN-slug> [acceptance args...]
# Builds HowlFrame and the app from the COMMITTED tree only (git archive HEAD into a
# fresh temp dir), then runs the frozen acceptance suite. Needs Bash, Git, tar, ordinary host utilities, Go and Python 3.
set -euo pipefail
app_dir="${1:?usage: clean_repro.sh <NN-slug>}"; shift
tool="${app_dir#*-}"
repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
git -C "$repo" archive HEAD | tar -x -C "$work"
app="$work/milestone1/$app_dir"
(cd "$work" && go build -o "$work/howlframe" .)
mkdir -p "$work/out"
"$work/howlframe" build "$app/src/$tool.howl" -o "$work/out/$tool.hfbc"
export HF_BIN="$work/howlframe" APP_ARTIFACT="$work/out/$tool.hfbc"
python3 "$app/tests/acceptance/run_acceptance.py" "$app/tests/acceptance/launcher.example.sh" "$@"
