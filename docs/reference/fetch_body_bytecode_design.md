# Fetch body on both bytecode compilers (#90)

Status: design spike. Docs only. #90 stays Partial. Promote stays DEFERRED.

This note is the design alternate Dev Lead Motoko flagged after the Assurance tip-lock. It does not implement a body, add an opcode, add a capability, or change production `-compile-bc`. The Owner authorizes an implementation later, as its own slice, on both bytecode compilers together.

The checklist that holds the current limit is `docs/reference/lowered_hfir_prod_flip_criteria.md`. The locked tip is `4d74dbcf9654caa05e0b1d9212b15bc5398359e3` (PR #71). Assurance on that SHA: Overall PASS, Promote DEFERRED (`docs/journals/2026-09-30_assurance_tip_lock_4d74dbcf.md`). The measurement record is `docs/journals/2026-09-30_lowered_hfir_prod_flip_tip_lock.md`.

## Status quo

`(fetch url method [body])` is one construct. The grant is `network`, the same name as `capability.ForConstruct("fetch")`.

| Path | What it does with a body |
| --- | --- |
| HFIR graph | `LowerAST` records a `url` edge, a `method` edge, and, when the call has a third argument, a `body` edge. The verifier requires `url` and `method`. `body` is optional. |
| `bytecode.CompileToBytecode` | Compiles child 1 (URL) and child 2 (method). Emits `FETCH`. Does not compile a fourth child. |
| `hfir.LowerToBytecode` | Compiles the `url` edge and then the `method` edge. Emits `FETCH`. Does not compile a `body` edge. A missing URL or method edge, or a method edge before the URL, is `HFIR_BYTECODE_UNSUPPORTED` and no `BCProgram`. |
| `OpFetch` | Name `FETCH`. Pops 2 (method, then URL). Pushes 1 (response bytes). Operand list empty. `IntOperand` is the zero value and is omitted from the JSON artifact. Capability `network`. |
| Bytecode VM | The grant check runs at the opcode, before `http.NewRequest`. The request body argument is `nil`. |
| Interpreter | The grant check runs before the call's children are evaluated. Three children send a nil reader. Four children send `strings.NewReader` of the evaluated body. |
| Generated Go | `howlFrameFetch` checks `network` before `http.NewRequest`. A missing body is a nil reader. A present body is `strings.NewReader` of that expression. |
| Generated JavaScript | `howlFrameFetch` checks `network` before `fetch`. A two-argument call leaves `init.body` unset. A third argument sets `init.body`, including `""`. |

The shared conformance form is `(fetch url method)` with no body (`tests/conformance/abi_v1/08_fetch_capability.howl`, `20_nested_fetch.howl`). `21_nested_multi_effect.howl` has one untaken `(fetch url "PUT" "phase2d-body-not-sent")`. That call has a `body` edge. Neither bytecode artifact contains the string, and the untaken branch does not run. `TestHFIRBytecodeFetchOperandOrder` sends that same string on a taken `PUT` and records an empty request body, because both compilers drop it. `TestProdFlipCriteriaLock` requires `OpFetch` to pop 2 with an empty operand list, and its `fetch_body` subtest requires the body string to be absent from both artifacts.

The model-adapter transport still rejects kind `fetch` with `HFIR_TRANSPORT_KIND`. #88 stays Pending. Wasm's infeasible set stays `exec`, `spawn_agent`, and `http_server_start`. `fetch` is not in that set, and this note does not add it.

## Why the body is an accepted shared limit

`FETCH` has a fixed stack arity. The VM pops two strings. If one compiler pushed a body and the other did not, the stacks would disagree: one path would underflow or leave a value, and HFIR↔AST parity on programs that pass a body would end. The operand list is empty on purpose. The body string is absent from both artifacts, and the tip-lock records that absence.

The interpreter, Go, and JavaScript already send a body when the call has one. The lag is the two bytecode compilers and the VM handler that consumes `FETCH`. Keeping the drop on both compilers is what makes the bytecode hosts match each other. Lifting it on one compiler would be a parity failure, which the checklist defers.

The checklist's promote rule is: the body stays off `OpFetch`, and a later body design is a separate change on both compilers together. The kill row treats an `OpFetch` body operand, or any other fetch-body implementation, as a kill when a PR uses it to justify a production flip. A body slice that does not flip `-compile-bc` is that separate change. A flip PR that picks up the body in order to flip is the kill.

## What a later slice has to preserve

These constraints apply to every option below. They are the contract the implementation slice has to meet. This note does not implement them.

* Both `bytecode.CompileToBytecode` and `hfir.LowerToBytecode` change in the same slice, and they emit the same operand shape for the same source.
* Compile order stays URL, then method, then the body when it is present. The VM pops in reverse, which is how `FETCH` already pops the method before the URL. `WRITE_FILE` is the same pattern: path, then data, then the opcode pops data first.
* Absent and empty stay different. No body means a nil reader on Go and an omitted `init.body` on JavaScript. A present `""` is `strings.NewReader("")` on Go and `init.body = ""` on JavaScript. Collapsing `""` into "no body" would change requests the AST hosts already send.
* The grant stays `network`. There is no new capability. Denial stays `CAPABILITY_DENIED` before `http.NewRequest` or `fetch`. The denial text omits the URL and the body, and the denial opens no connection. On the bytecode VM the grant check already runs at the opcode, after the instructions that produced the URL and the method. A body value produced by earlier instructions is the same order. The implementation slice keeps the check before the request.
* A non-string URL or method on the VM is `TYPE_ERROR` via `popCheckedString`. A popped body uses that same check. The implementation slice adds a conformance case for a non-string body on the bytecode VM. It leaves the interpreter, Go, and JavaScript conversions as they are unless that case shows a host mismatch the Owner wants closed in the same slice.
* The HFIR `body` edge is compiled only when it is the third data edge and its name is `body`. A fetch whose edges are missing or reordered stays `HFIR_BYTECODE_UNSUPPORTED` and no `BCProgram`. The AST path compiles the fourth child only when the call has one.
* The two-argument conformance cases stay body-absent: `fetch_denied`, `fetch_granted`, `nested_fetch_denied`, `nested_fetch_granted`. The untaken body in `nested_multi_effect` still must not be sent. A new granted case records the request body on the interpreter, the production bytecode VM, experimental `-compile-hfir-bc`, Go, and JavaScript. A new denial case with a body still opens no connection and does not echo the body.
* `TestHFIRBytecodeFetchOperandOrder` currently expects an empty server body for `"phase2d-body-not-sent"`. The implementation slice changes that expectation because the taken `PUT` starts sending the string. That update is the point of the slice.
* `TestProdFlipCriteriaLock` currently requires pops 2, an empty operand list, and an artifact without the body string. The implementation slice updates those assertions in the same change as the compilers. This spike leaves the test alone.
* Either compiler changing expires the Assurance tip-lock. The body slice cites that expiry, does not claim the old lock, and does not flip `-compile-bc`. A new lock is a later measurement, taken when the Owner asks for one. The flip PR, if any, cites a fresh lock and still lists whatever accepted limits remain.
* The transport allow-list stays the Phase-1 adapter subset. `nodeRoles` does not gain `fetch`. Wasm does not gain a fetch import. #106 stays the do-not-build linker decision.

## Design options

Three shapes can carry a body. All three are future work. None of them is authorized by this note.

### Option A — presence flag on the existing `FETCH`

Keep the opcode `FETCH` and the capability `network`. Follow the `EXEC` precedent: `EXEC` carries `IntOperand` as a count and uses `Pops: -1`.

`IntOperand` 0 means the call has no body. The VM pops the method and then the URL and passes a nil reader. That is the zero value already stored on every `FETCH` in current artifacts (`int_operand` is `omitempty`, so 0 is absent from the JSON). Two-argument programs keep today's encoding.

`IntOperand` 1 means the call has a body. Both compilers emit the body instructions after the method. The VM pops the body, then the method, then the URL. The body is a string, and `""` is a present empty body.

The registry row in that slice becomes operand list `[int64]`, pops variable, pushes 1, capability `network`. The checklist sentence "empty operand list" and "pops 2" is rewritten in that same slice, because the accepted limit is being lifted by the Owner's separate change.

This option puts a body operand on `OpFetch`. Inside a production-flip PR, that shape is the kill row. The implementation slice is not a flip PR.

### Option B — a second opcode for the three-argument call

Leave `OpFetch` as it is locked today: pops 2, empty operand list, capability `network`. Add one opcode, pops 3 (body, method, URL), pushes 1, capability `network`, empty operand list. A concrete name is `FETCH_WITH_BODY`. The grant name stays `network`. This is not a new capability.

`bytecode.CompileToBytecode` emits `FETCH` for three children and the new opcode for four. `LowerToBytecode` emits `FETCH` when the node has the URL and method edges only, and the new opcode when the third edge is `body`. The VM's existing `FETCH` handler stays on a nil reader. The new handler pops the body string and passes it to `http.NewRequest`.

Current two-argument artifacts stay valid without a flag. The cost is two opcodes for one source form, and a conformance matrix that has to cover both. A new opcode used to justify a production flip is also a kill. Fetch body is an accepted limit, not a promote-blocker name, so this opcode does not clear the fence.

### Option C — always pop 3, with an absent sentinel

Change `OpFetch` to pop 3 unconditionally. When the source has no body, both compilers push nil before `FETCH`. When the source has a body, they push that value, including `""`. The VM treats nil as a nil reader and a string as the body.

Absent and empty stay distinct, and there is still one opcode. Every existing `FETCH` artifact underflows, because those artifacts push only the URL and the method. The registry pops field changes from 2 to 3, which fails `TestProdFlipCriteriaLock` until that test is updated in the implementation slice. There is no zero-value encoding that means "keep today's pop 2".

### Constant operand, and why it does not cover the call

A `StringOperand` on `FETCH` could store a literal body. The hosts already evaluate a body expression (`str_join`, a variable, a call). An operand string would send literals and would still drop computed bodies, which is the split this design is trying to close. A constant operand is not a sufficient option on its own. Options A and B already carry a computed string on the stack.

## Non-goals

* No implementation in this change. No edit under `internal/bytecode`, `internal/hfir`, `internal/vm`, `internal/backend`, or `howlframe.go`.
* No new opcode and no new capability in this change.
* No production `-compile-bc` flip. The `*compileBc` branch stays `bytecode.CompileToBytecode`.
* No conformance JSON edits, no lock-test edits, and no change to the tip-lock SHA `4d74dbcf9654caa05e0b1d9212b15bc5398359e3`.
* #90 stays Partial. Promote stays DEFERRED. One lowered graph for every host is still Phase 2.
* This note does not choose an option for the Owner. The recommendation below is for review.

## Recommendation

Non-binding. For Motoko's review, and for the Owner to accept or replace when kicking the implementation slice.

Prefer option A. One opcode, the existing `network` grant, and `IntOperand` 0 preserves the artifacts the tip-lock already measured. `IntOperand` 1 is the present body, including `""`. Both compilers emit that flag together. The VM reads it in the same slice and still denies before `http.NewRequest`.

Option B is the alternative if the review wants `OpFetch`'s locked shape to stay byte-for-byte unchanged and will accept a second opcode. Option C is the alternative if a single unconditional arity is worth invalidating existing `FETCH` artifacts.

The implementation slice, when the Owner kicks it:

1. Lands option A (or the reviewed replacement) in `bytecode.CompileToBytecode`, `hfir.LowerToBytecode`, and the `OpFetch` VM handler together.
2. Adds the granted body case and the denial-with-body case on the hosts that already mediate `fetch`. Updates the operand-order expectation that currently records an empty body, and updates the `fetch_body` lock assertion, in that same slice.
3. Rewrites the Fetch body row of the checklist so it describes the shape that shipped. Leaves the promote-blocker fence, the Decision (defer), and the kill rule for flip-justifying body work in place.
4. Does not flip `-compile-bc`. States that the tip-lock `4d74dbcf` has expired because the compilers changed. Does not mark #90 Done.

The next build slice on the promote path is still `attr_escape` parity (and `regex_match` onto `REGEX_MATCH`) unless the Owner says otherwise. Those names are the fence. The body is the accepted limit, and it stays a separate change.
