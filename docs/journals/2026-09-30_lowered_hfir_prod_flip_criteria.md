# Lowered HFIR ABI, prod-flip criteria

## Why this slice

Phases 2a–2e and 3a–3d, and the dogfood comparisons through PR #69, show experimental `-compile-hfir-bc` matching production `-compile-bc` on a bounded subset. Those matches are not a production flip. #90 stays Partial. This slice writes the kill, defer, and promote checklist for eventually pointing `-compile-bc` at `hfir.LowerToBytecode`, and it leaves the flag on `bytecode.CompileToBytecode`.

## Starting SHA

`a9f00bcc91680abe505616fb9b9ed6e643ffa3bc` on `origin/main` (PR #69). That commit is the baseline the checklist was written against. It is not an Assurance tip-lock release.

## Decision

Defer. The checklist is `docs/reference/lowered_hfir_prod_flip_criteria.md`.

Production `-compile-bc` stays `runHFIRGate` and then `bytecode.CompileToBytecode`. Experimental `-compile-hfir-bc` stays the dogfood path until the Owner authorizes a flip PR. A later flip that meets the promote rows still leaves #90 Partial, because one lowered graph for every host is still Phase 2.

## What the checklist requires before a flip

Promote needs three bodies of evidence on one Assurance tip-lock: HFIR↔AST parity for every program production already accepts, mediated host effects for `env`, `exec`, `read_file`, `write_file`, `mkdir`, and `fetch` on the hosts that already mediate them, and a named `main` SHA whose production results are the oracle. The flip diff is one emission source in the `*compileBc` branch. `-compile-hfir-bc` remains spelled the same way in that PR.

The promote-blocker fence is every `construct.Supported` name with a `compileNode` case and no `LowerToBytecode` case. At this tip that fence has thirty names, including `regex_match`, `html_escape`, and `attr_escape`. `tests/parity/07_strings.howl` and `tests/parity/13_html_escape.howl` are the rejected parity files. Stores, the HTTP server, `spawn`, database access, model calls, `time_now`, `sleep`, and `read_line` are the same class of blocker. That thirty-name count is the baseline at `a9f00bcc`. A later dogfood slice, `docs/journals/2026-09-30_lowered_hfir_abi_html_escape.md`, lowers `html_escape` onto `HTML_ESCAPE` on `-compile-hfir-bc` only, so that name leaves the fence. `regex_match` and `attr_escape` stay in it. The living fence is the checklist's `promote-blockers` block. The slice does not flip production, and #90 stays Partial.

Accepted limits, which a flip keeps and does not implement: the optional fetch body stays off `OpFetch` on both bytecode compilers; `match` stays unsupported with empty control edges; `try` keeps emitting `TRY_LET` with empty control edges; Wasm stays the three-kind `HFIR_TARGET_INFEASIBLE` set; the module linker stays the #106 refusal; the model-adapter transport keeps rejecting `defun`, `call`, `return`, `while`, `for`, `read_file`, `write_file`, `mkdir`, `exec`, `fetch`, and the escape and regex kinds with `HFIR_TRANSPORT_KIND`. #88 stays Pending.

Kill the track if a proposed PR adds an opcode, a capability, Wasm, a linker, or a fetch body in order to flip, or if it splits production emission or marks #90 Done off the current subset.

## How the criteria stay locked

`TestProdFlipCriteriaLock` recomputes the fence from the construct table and the two compiler switches. It reads the checklist's `promote-blockers` fence and requires the same names. It also requires the `*compileBc` branch to call `bytecode.CompileToBytecode` and the `*compileHfirBc` branch to call `hfir.LowerToBytecode`.

## What this does not do

No change to production `-compile-bc`. No new opcode, capability, Wasm host, or module linker. No fetch-body implementation. No edit to `nodeRoles`. `match` and `try` stay without control edges. #90 stays Partial.
