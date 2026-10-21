#!/usr/bin/env bash
# Archive the committed framework and two sanitized sibling files. Dependencies
# are host tools; this is environment isolation, not an OS/security sandbox.
set -euo pipefail
app=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
repo=$(cd -- "$app/../.." && pwd)
board=$(cd -- "$repo/../howlboard" && pwd)
root=$(mktemp -d)
trap 'rm -rf -- "$root"' EXIT
mkdir -p "$root/howlframe" "$root/howlboard/backend" "$root/howlboard/data/fixtures" "$root/home" "$root/tmp" "$root/gopath" "$root/gocache" "$root/gomodcache"
git -C "$repo" archive HEAD | tar -xf - -C "$root/howlframe"
git -C "$board" show HEAD:backend/server.howl > "$root/howlboard/backend/server.howl"
git -C "$board" show HEAD:data/fixtures/ledger_sample.jsonl > "$root/howlboard/data/fixtures/ledger_sample.jsonl"
board_commit=$(git -C "$board" rev-parse HEAD)
# Playwright is an explicitly declared host dependency. Preserve only its package
# search path and browser installation, not the owner's HOME or general env.
python_packages=$(python3 -c 'import site; print(site.getusersitepackages())')
browser_cache=${PLAYWRIGHT_BROWSERS_PATH:-$HOME/.cache/ms-playwright}
printf 'Framework archive commit: %s\nSibling commit: %s\n' "$(git -C "$repo" rev-parse HEAD)" "$board_commit"
env -i PATH=/usr/bin:/bin GOTOOLCHAIN=local HOME="$root/home" TMPDIR="$root/tmp" GOPATH="$root/gopath" GOCACHE="$root/gocache" GOMODCACHE="$root/gomodcache" PYTHONPATH="$python_packages" PLAYWRIGHT_BROWSERS_PATH="$browser_cache" HOWLBOARD_COMMIT="$board_commit" NO_PROXY=127.0.0.1,localhost LANG=C.UTF-8 bash "$root/howlframe/milestone3/evidencedesk/scripts/test.sh"
