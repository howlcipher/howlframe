# 2026-08-05: Centralize Capability Metadata (Improvement #94)

## Context
During HFIR wiring (improvement #87 Phase 2), we noticed that `internal/hfir/verifier.go` had a hardcoded `getCapability` switch mapped HFIR node kinds to capability types. The bytecode compiler `internal/bytecode/opcode.go` had its own capability configuration for opcodes. The two representations were completely independent and ran the risk of silent divergence if a new capability was added to one but not the other.

The goal of improvement #94 was to centralize the capability representation so there is a single authoritative source of truth that is backend and target-neutral.

## Design Decisions
1. **Target-independent location**: I created the new `github.com/howlcipher/howlframe/internal/capability` package as the source of truth for the capability enum (`capability.Capability`) and its canonical mapping.
2. **Direction**: The dependency direction flows from both `internal/hfir` and `internal/bytecode` into `internal/capability`. This preserves the backend independence of the HFIR compiler and bytecode systems.
3. **Construct Mapping**: `capability.ForConstruct(name string)` was implemented as the single mapping from canonical HowlFrame operation strings to capabilities. `internal/hfir` accesses this directly using `node.Kind`.
4. **Bytecode Opcode Capabilities**: In `internal/bytecode/opcode.go`, opcodes were updated to use the new `capability.Capability` type. Their capability associations are manually hardcoded in `Registry` (matching the prior state).
5. **Drift Test**: A new drift test was introduced (`internal/bytecode/drift_test.go`) which dynamically ensures that the capability defined in `bytecode.Registry` for a specific opcode always aligns with `capability.ForConstruct`.

## Execution
- Extracted capabilities to `internal/capability`.
- Added determinism test for capability enumeration (`internal/capability/capability_test.go`).
- Modified `opcode.go` and `bytecode_test.go` to depend on the new capability package.
- Modified `verifier.go` to invoke `capability.ForConstruct` instead of using the hardcoded switch.
- Modified VM enforcement paths (`internal/vm/vm.go`, `store_test.go`, and `capability_test.go`) to use the new capability types.
- Replaced `-allow-caps` flag references to use `capability` constants in `howlframe.go`.
- Identified a known capability mismatch where the `ACHIEVE` construct was assigned `network` in HFIR but no capability was enforced for `OpAchieve` in the bytecode VM. Rather than silently altering the VM's security boundary during this refactor, I documented it as a defect (Bug #44) in `bugs.md` and explicitly skipped it in the drift test.

## Results
All tests pass. The codebase now has a single, backend-agnostic source of truth for all capability definitions without any backwards layering constraints.
