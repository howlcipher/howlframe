#!/usr/bin/env bash
# Reference launcher for the ledgerlens acceptance suite (contract: REQUIREMENTS.md section 1.2).
#   launcher.example.sh GRANTS [APP_ARG...]
# Environment: HF_BIN (howlframe binary, default: howlframe), APP_ARTIFACT (path to ledgerlens.hfbc),
#              HF_RUN_FLAGS (extra runner flags, e.g. "--max-instructions 5000").
HF_BIN="${HF_BIN:-howlframe}"
: "${APP_ARTIFACT:?set APP_ARTIFACT to the built .hfbc}"
grants="$1"; shift
flags=()
if [ -n "${HF_RUN_FLAGS:-}" ]; then read -r -a flags <<<"$HF_RUN_FLAGS"; fi
if [ -n "$grants" ]; then
  exec "$HF_BIN" run --allow-caps "$grants" "${flags[@]}" "$APP_ARTIFACT" "$@"
else
  exec "$HF_BIN" run "${flags[@]}" "$APP_ARTIFACT" "$@"
fi
