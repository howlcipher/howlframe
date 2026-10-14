#!/usr/bin/env bash
# Reference launcher for the ticketdesk acceptance suite (contract: REQUIREMENTS.md R1.1).
#   launcher.example.sh GRANTS [APP_ARG...]
# Environment: HF_BIN (howlframe binary, default: howlframe), APP_ARTIFACT (path to ticketdesk.hfbc),
#              HF_RUN_FLAGS (runner flags, e.g. "--max-request-bytes 2048 --access-log").
# TICKETDESK_* variables are inherited unchanged. exec: SIGTERM reaches the runner.
HF_BIN="${HF_BIN:-howlframe}"
: "${APP_ARTIFACT:?set APP_ARTIFACT to the built ticketdesk.hfbc}"
grants="$1"; shift
flags=()
if [ -n "${HF_RUN_FLAGS:-}" ]; then read -r -a flags <<<"$HF_RUN_FLAGS"; fi
if [ -n "$grants" ]; then
  exec "$HF_BIN" run --allow-caps "$grants" "${flags[@]}" "$APP_ARTIFACT" "$@"
else
  exec "$HF_BIN" run "${flags[@]}" "$APP_ARTIFACT" "$@"
fi
