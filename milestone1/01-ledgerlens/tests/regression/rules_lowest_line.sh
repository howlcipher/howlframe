#!/usr/bin/env bash
# Regression for review finding LL-1: check-rules reports the LOWEST-line error even when an
# unknown-budget error (needs the whole file) precedes a later syntax error.
set -euo pipefail
L="${1:?launcher}"; d="$(mktemp -d)"; trap 'rm -rf "$d"' EXIT
printf '[categories]\nA = x\n[budgets]\nB = 5\n\nbad line\n' > "$d/r.ini"
out="$("$L" "filesystem:read=$d" check-rules "$d/r.ini" 2>&1 || true)"
case "$out" in *"rules: line 4: "*) echo PASS;; *) echo "FAIL: $out"; exit 1;; esac
# a later category line that defines the budget's category makes the budget valid again
printf '[budgets]\nB = 5\n[categories]\nB = x\nbad line\n' > "$d/r2.ini"
out="$("$L" "filesystem:read=$d" check-rules "$d/r2.ini" 2>&1 || true)"
case "$out" in *"rules: line 5: "*) echo PASS;; *) echo "FAIL: $out"; exit 1;; esac
