#!/usr/bin/env bash
set -euo pipefail
app=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
repo=$(cd -- "$app/../.." && pwd)
mkdir -p "$app/build"
compiler=${HOWLFRAME_BIN:-$app/build/howlframe}
if [[ -z ${HOWLFRAME_BIN:-} ]]; then (cd "$repo" && go build -o "$compiler" howlframe.go); fi
"$compiler" build "$app/app/backend.howl" -o "$app/build/backend.hfbc"
"$compiler" build "$app/app/projector.howl" -o "$app/build/projector.hfbc"
"$compiler" build "$app/app/frontend.howl" --target js -o "$app/static/app.js"
