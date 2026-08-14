# Goal
Execute the CURRENT ChangeOps HowlFrame policy through HowlFrame's direct HFIR execution architecture.

# Starting HowlFrame SHA
6110cc687b038467fa641dd9041cc187d14279e4

# ChangeOps SHA
41bb22153c0f6cecf0b82efb1b5957330a4b32fc

# HowlBoard SHA
870df7d32b28d8088ea605c3af3cd75897e877ff

# Current direct-HFIR subset
Phase 1 executable subset included program, sequence, const, symbol, let, set, if, binary, list, dict, print, etc.

# ChangeOps construct inventory
`read_file`, `parse_json`, `try_let`, `catch`, `for`, `cli_args` (with indices), `is_nil`.

# Missing semantic forms
All the constructs in the inventory were missing from the direct-HFIR subset.

# Architecture decisions
1. **Semantic HFIR Lowering**: We extended `lowerSemanticList` to map the missing AST constructs explicitly to semantic HFIR nodes with named data/control edges rather than an opaque AST.
2. **Deterministic Verification First**: Before adding direct lowering, we extended `verifier.go` with strict structural constraints for the new nodes (e.g. `try` must have `expression`, `success_body`, and `catch`).
3. **Direct Bytecode Lowering**: We added support to map the new HFIR semantic nodes directly to their `BCInstruction` equivalents in `internal/hfir/bytecode.go`.
4. **Experimental Compiler Flag**: We exposed the new compilation path via an explicit, experimental `-compile-hfir-bc` flag in the HowlFrame CLI, leaving the public `-compile-bc` default unchanged.

# Implementation
- **HFIR Lowering**: `internal/hfir/lowering.go` now explicitly maps `try_let`, `catch`, `for`, `read_file`, `parse_json`, `is_nil`, and `cli_args` to named edges.
- **Verifier**: `internal/hfir/verifier.go` checks required semantic roles. Added explicit test coverage in `verifier_test.go` and verified capability tracking properly assigns `filesystem` effects to `read_file`.
- **Bytecode Lowering**: `internal/hfir/bytecode.go` directly targets the `bytecode.BCProgram` backend.
- **Semantic Repair**: A test in `internal/hfir/repair_test.go` proves HFIR's viability for autonomous fixes: it locates a `try` node missing its `catch` edge, injects a fallback catch node, and passes validation.

# Differential evidence
We compiled the unmodified `changeops.howl` using both the legacy AST compiler (`-compile-bc`) and the direct HFIR path (`-compile-hfir-bc`).
A test script (`test_changeops.sh`) successfully verified identical behavior across 10 execution paths including `inspect`, `validate`, `record_release_ready` (valid/invalid), `create_release_candidate` (approved/unapproved/stale), and `rollback_release_candidate`.

# Consumer evidence
ChangeOps successfully executes unmodified over the direct HFIR path with all policy decisions identical.

# Security evidence
1. Capability tracking remained correctly inferred directly from HFIR via `read_file` mappings. The bytecode correctly aborted execution when missing the `filesystem` capability over both legacy and HFIR paths.
2. HowlBoard was analyzed and found to have a severe `set_html` XSS risk due to string concatenation with untrusted DOM inputs. This is documented in `docs/howlboard_xss_set_html_issue.md`.

# Remaining gaps
The HFIR compiler path does not yet support:
- persistent state (`store_open`, `store_get`, etc.)
- functions (`defun`, `call`, `return`)
- web backend/frontend features (`http_server`, `route`, `web_app`, `fetch`, `dom_query`, `set_html`)

This is tracked in `docs/external_consumer_hfir_gap_matrix.md`.

# Final recommendation
Direct HFIR execution is proven and reliable for Phase 2 consumers (ChangeOps). Phase 2B should target HowlBoard, requiring `defun`, `call`, `return`, `route`, and persistent `store` operations to be safely added to HFIR's supported subset. Meanwhile, HowlBoard's `set_html` XSS vulnerability must be resolved.
