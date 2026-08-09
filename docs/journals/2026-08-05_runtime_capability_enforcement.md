# Runtime capability enforcement Phase 1: VM-level allow/deny gate
Date: 2026-08-05

## Task Summary
Implemented improvement #79: a runtime capability policy for `internal/vm.BCVM`, closing the gap between the bytecode registry's `Capability` metadata (documentation/schema-only) and actual VM enforcement.

## Details
- `BCVM` gained an `AllowedCaps []bytecode.Capability` field; `RunBytecode` now takes an `allowedCaps` parameter and threads it through.
- `BCVM.run`'s instruction loop checks `bytecode.Registry[op].Capability` before dispatch and panics with a structured `CAPABILITY_DENIED` `VMError` (via the existing `NewRuntimeError`) if the capability isn't in the allow-list. Default is fail-closed: a nil/empty allow-list denies every capability-gated instruction; `CapNone` instructions always run.
- `howlframe.go` added `-allow-caps` (comma-separated capability names, validated against the known set, must precede the positional input file per the existing `flag`-package-stops-at-first-positional-arg convention already documented for `-o`).
- Tests: `internal/vm/capability_test.go` covers allowed/denied/denied-by-other-caps for all five capability kinds plus a CapNone-always-runs case; `howlframe_test.go`'s `TestRunBytecodeAllowCapsFlagGatesCapabilities` covers the CLI flag end-to-end (default denial, granted access with the flag, rejection of an unrecognized capability name).
- Fixed fallout: `internal/vm/store_test.go`'s `newStoreTestVM` now grants `CapDatabase` (store opcodes are capability-gated); `tools/difftest/main.go`'s `-run-bc` step now passes all five capabilities so its backend-parity fixtures (several of which read env vars) aren't broken by the new default-deny behavior.
- Docs: added a "Capability Security" section to README.md.

## Not done
- The knowledge library's `howlframe_transpiler` skill file was intentionally left untouched — per established convention it's only edited from sessions working directly in `ai_knowledge_library`, never from a HowlFrame repository session. Flagging that its "Runtime capability enforcement" status is now stale there.

## Next Steps
Phase 2 (scoped/granular permissions, e.g. specific env keys or hosts) is unblocked now that the coarse allow/deny gate exists.
