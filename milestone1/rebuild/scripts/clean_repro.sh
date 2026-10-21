#!/usr/bin/env bash
# Committed fresh candidates; host tools, not an OS sandbox.
set -euo pipefail
repo=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../../.." && pwd)
work=$(mktemp -d)
trap 'rm -rf -- "$work"' EXIT
mkdir -p "$work/tree" "$work/home" "$work/tmp" "$work/artifacts"
git -C "$repo" archive HEAD | tar -xf - -C "$work/tree"
printf 'Framework archive commit: %s\n' "$(git -C "$repo" rev-parse HEAD)"
env -i PATH=/usr/bin:/bin LANG=C.UTF-8 GOTOOLCHAIN=local \
  HOME="$work/home" TMPDIR="$work/tmp" GOCACHE="$work/go-build" \
  GOMODCACHE="$work/go-mod" HOWL_FORBID_LOCAL_INFERENCE=1 \
  bash -s -- "$work/tree" "$work/artifacts" <<'VERIFY'
set -euo pipefail
cd -- "$1"
artifacts=$2
go version
python3 --version
go build -o "$artifacts/howlframe" .
for directory in 01-ledgerlens 02-healthwatch 03-taskflow; do
  name=${directory#*-}
  "$artifacts/howlframe" build "milestone1/rebuild/$directory/src/$name.howl" -o "$artifacts/$name.hfbc"
  HF_BIN="$artifacts/howlframe" APP_ARTIFACT="$artifacts/$name.hfbc" \
    python3 "milestone1/$directory/tests/acceptance/run_acceptance.py" \
    "milestone1/$directory/tests/acceptance/launcher.example.sh"
done
HF_BIN="$artifacts/howlframe" M1_ARTIFACT_DIR="$artifacts" \
  python3 milestone1/tests/regression/fresh_boundaries.py
HF_BIN="$artifacts/howlframe" APP_ARTIFACT="$artifacts/healthwatch.hfbc" \
  python3 milestone1/rebuild/02-healthwatch/tests/supplemental.py
VERIFY
