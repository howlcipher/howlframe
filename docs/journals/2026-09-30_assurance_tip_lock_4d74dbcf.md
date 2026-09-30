# Assurance tip-lock of `4d74dbcf`

## Why this slice

Assurance (Lain) verified the tip-lock of `main` SHA `4d74dbcf9654caa05e0b1d9212b15bc5398359e3` against `docs/reference/lowered_hfir_prod_flip_criteria.md` and `TestProdFlipCriteriaLock`. This journal records that verdict. The checklist record remains `docs/journals/2026-09-30_lowered_hfir_prod_flip_tip_lock.md`. The logs are `evidence/howl-90-assurance-tip-lock-4d74dbcf/`.

Overall: PASS. Promote: DEFERRED. Decision: Defer. The lock is not a promote. It does not authorize a flip. #90 stays Partial.

## Locked tip

`4d74dbcf9654caa05e0b1d9212b15bc5398359e3` (`4d74dbcf`).

Commit message: `feat: experimental HFIR html_escape onto existing HTML_ESCAPE (#71)`.

Assurance fetched `origin/main` and checked out that SHA detached. `git rev-parse HEAD` matched the tip, and `origin/main` was the same commit. The run used the existing clone at that SHA. A separate shallow clone was not made.

Recorded 2026-09-30, about 15:52–15:56 EDT (America/Detroit). Assurance did not flip production `-compile-bc`, did not mark #90 Done, and did not invent secrets.

## Environment

From `evidence/howl-90-assurance-tip-lock-4d74dbcf/env.txt`:

| Tool | Path | Version |
| --- | --- | --- |
| node | `/usr/bin/node` | v20.19.2 |
| go | `/usr/bin/go` | go1.24.4 linux/amd64 |

Node was present, so the JavaScript host ran. The result is not `BACKEND_UNSUPPORTED`.

## Suite

All three commands exited 0 on the locked tip. Logs:

* `evidence/howl-90-assurance-tip-lock-4d74dbcf/suite-difftest.log` — `ok github.com/howlcipher/howlframe/tools/difftest 22.035s`
* `evidence/howl-90-assurance-tip-lock-4d74dbcf/suite-vm.log` — `ok github.com/howlcipher/howlframe/internal/vm 0.061s`
* `evidence/howl-90-assurance-tip-lock-4d74dbcf/suite-prodflip.log` — `ok github.com/howlcipher/howlframe 0.006s`

```
go test ./tools/difftest -count=1 -timeout 900s -run 'TestLoweredHFIRABIConformance|TestHFIRBytecodeSupportedParity|TestHFIRBytecodeRejectsUnsupportedParity'
go test ./internal/vm -count=1 -timeout 900s -run 'TestHFIRBytecode'
go test . -count=1 -timeout 180s -run 'TestProdFlipCriteriaLock'
```

### QA

`TestLoweredHFIRABIConformance` passed, including mediated grant and denial for `env`, `exec`, `read_file`, `write_file`, `mkdir`, and `fetch`, the nested control fixtures, `nested_multi_effect_denied`, `nested_multi_effect_partial`, `nested_multi_effect_granted`, and `html_escape` plus `html_escape_type_error`. Hosts include `interpreter`, `bytecode`, `hfir_bytecode`, `go`, and `javascript`.

`TestHFIRBytecodeSupportedParity` passed on the eleven supported files (`01`–`06`, `08`–`12`). `TestHFIRBytecodeRejectsUnsupportedParity` passed on `07_strings.howl` and `13_html_escape.howl`. Those two files stay in `hfirRejectedParity`. That non-empty list defers promote. It does not fail the tip-lock.

`TestHFIRBytecode` passed, including HTML_ESCAPE fixture and type-error parity with the AST, operand order, nested multi-effect, and capability denial. `TestProdFlipCriteriaLock` passed, including every transport subtest, `fetch_body`, and empty `match` / `try` control edges.

Production `*compileBc` still calls `bytecode.CompileToBytecode(root)`. Experimental `*compileHfirBc` still calls `hfir.LowerToBytecode(graph)`.

### Security

Denial class on the mediated effects is `CAPABILITY_DENIED` before the effect. `OpFetch` is `FETCH`, pops 2, has an empty operand list, and is `network`. The fetch body string is absent from both compilers' artifacts (`TestProdFlipCriteriaLock/fetch_body` passed). The body stays off `OpFetch`.

`hfir.WasmInfeasibleKinds` stays `exec`, `spawn_agent`, and `http_server_start`. The model-adapter transport rejects still hold (`HFIR_TRANSPORT_KIND`). Module linking stays on the AST. There is no VM module opcode. #106 stays closed.

No kill-shape on the tip: no new opcode or capability, no Wasm growth, no linker, no `OpFetch` body operand, no second production emitter, no dropped production blocker, no widened `nodeRoles`, and #90 is not marked Done. `html_escape` reuses the existing `HTML_ESCAPE` opcode. `regex_match` and `attr_escape` still emit on the AST path.

## Fence after `html_escape`

The promote-blocker fence is still non-empty: 29 names. `html_escape` is outside it and emits the existing `HTML_ESCAPE` opcode with parity. `regex_match` and `attr_escape` remain in the fence. Experimental lowering of those two names returns `HFIR_BYTECODE_UNSUPPORTED` and no `BCProgram`.

`hfirRejectedParity` is `07_strings.howl` (`regex_match`) and `13_html_escape.howl` (`attr_escape`; that file also calls `html_escape`, which now lowers).

## Verdict

Overall: PASS. The suite is green, the oracle still holds, and there is no kill-shape.

Promote: DEFERRED. The reasons are the non-empty promote-blocker fence, the non-empty `hfirRejectedParity` (`07_strings.howl`, `13_html_escape.howl`), and the absence of an Owner flip PR that cites this lock.

#90 stays Partial. One lowered graph for every host is still open. Production `-compile-bc` stays `bytecode.CompileToBytecode`. Experimental `-compile-hfir-bc` stays the dogfood path until the Owner authorizes a flip PR.

## What this does not do

No production `-compile-bc` flip. No new opcode or capability. No Wasm host. No module linker. No fetch-body implementation. This journal does not mark #90 Done.
