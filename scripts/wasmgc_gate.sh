#!/usr/bin/env bash
# The local regression gate for the WebAssembly GC backend, run before every merge of a change that touches it
# (docs/design/wasm_gc_backend.md, "Verification"). It runs what CI runs and what CI cannot:
#
#   HOWLSCENE_HF=/path/to/howlscene-hf scripts/wasmgc_gate.sh
#
# 1. gofmt, go vet and go build over the module.
# 2. go test ./... on the default toolchain, and again on GOTOOLCHAIN=${HF_LATEST_GO:-go1.27.2} (CI's latest-go job
#    uses Go "stable"; the VM must own any text newer Go changes).
# 3. The HowlScene HF gates (TestHowlScene*: the M1 rendering gate, the backup gate and the M2 core-tests gate),
#    which CI cannot run because HowlScene HF is a private repository.
#
# The WebAssembly tests skip a leg when node, wasmtime or wasm-tools is missing, and the gates skip without
# HOWLSCENE_HF, so a run without them passes while testing less. This script fails instead: it requires every tool
# up front, and any test skipped for a missing tool or checkout fails the gate. Every other skip is listed.
set -euo pipefail

repo=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
cd "$repo"
work=$(mktemp -d)
trap 'rm -rf -- "$work"' EXIT
latest=${HF_LATEST_GO:-go1.27.2}

fail() { echo "wasmgc gate: $*" >&2; exit 1; }

for tool in go node wasmtime wasm-tools python3; do
  command -v "$tool" >/dev/null || fail "$tool is not installed"
done
[[ -n ${HOWLSCENE_HF:-} && -d $HOWLSCENE_HF/tests/core ]] || fail "set HOWLSCENE_HF to a HowlScene HF checkout"
echo "tools: $(go version | cut -d' ' -f3), node $(node --version), $(wasmtime --version | cut -d' ' -f1-2), $(wasm-tools --version)"
echo "HowlScene HF: $HOWLSCENE_HF at $(git -C "$HOWLSCENE_HF" rev-parse --short HEAD)"

unformatted=$(gofmt -l .)
[[ -z $unformatted ]] || fail "not gofmt-formatted: $unformatted"
go vet ./...
go build ./...

# skips FILE: the tests a go test -json run skipped, as "package test: reason".
skips() {
  python3 - "$1" <<'EOF'
import json, sys
output, skipped = {}, []
for line in open(sys.argv[1]):
    try:
        e = json.loads(line)
    except ValueError:
        continue
    key = (e.get("Package"), e.get("Test"))
    if e.get("Action") == "output" and key[1]:
        output.setdefault(key, []).append(e["Output"])
    elif e.get("Action") == "skip" and key[1]:
        reason = [l.strip() for l in output.get(key, []) if ": " in l and not l.lstrip().startswith(("=== ", "--- "))]
        skipped.append(f"{key[0]} {key[1]}: {reason[-1] if reason else '(no reason)'}")
print("\n".join(skipped))
EOF
}

status=0
runs=0
run() { # run LABEL COMMAND...: a go test command, run with -json
  local label=$1; shift
  runs=$((runs + 1))
  local log="$work/run$runs.json"
  echo "== $label"
  if ! env "$@" -json >"$log" 2>&1; then
    grep -h '"Action":"fail"' "$log" | python3 -c 'import json,sys
for l in sys.stdin:
    e=json.loads(l); print("FAIL", e.get("Package"), e.get("Test") or "")' || true
    grep -h '"Output"' "$log" | python3 -c 'import json,sys
for l in sys.stdin:
    o=json.loads(l).get("Output","")
    if "build failed" in o or "cannot" in o or "panic:" in o: print(o, end="")' | head -20 || true
    status=1
  fi
  local s
  s=$(skips "$log")
  if [[ -n $s ]]; then
    echo "skipped:"
    sed 's/^/  /' <<<"$s"
    if grep -E '/(internal/backend/wasmgc|tools/difftest) ' <<<"$s" | grep -qE 'not installed|HOWLSCENE_HF'; then
      echo "a test was skipped for a missing tool or checkout" >&2
      status=1
    fi
  fi
  echo "   $(grep -c '"Action":"pass","Package":"[^"]*","Test"' "$log") tests passed"
}

run "go test ./... ($(go version | cut -d' ' -f3))" go test ./...
run "go test ./... ($latest)" GOTOOLCHAIN="$latest" go test ./...
run "HowlScene HF gates" HOWLSCENE_HF="$HOWLSCENE_HF" go test -count=1 -run '^TestHowlScene' ./internal/backend/wasmgc/

if [[ $status -ne 0 ]]; then
  fail "FAILED"
fi
echo "wasmgc gate: passed"
