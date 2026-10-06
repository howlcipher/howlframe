# 07: Security and authority findings

The thesis rests on one claim: *intent is not authority*. A program the
model proposes should get no effect the runner did not grant. This file
lists every way this review found that the claim does not yet hold, how
each was checked, and what changed.

Severity is judged for the stated use: running a **model-proposed program**
under a runner grant. It is not judged for a trusted developer running
their own code.

| ID | Finding | Severity | Status | How verified |
| --- | --- | --- | --- | --- |
| S1 | `CONFIDENCE`, `NEURAL_CIRCUIT`, `EPHEMERAL_CIRCUIT` reached `127.0.0.1:11434` under an **empty grant** on `-run-bc`. `ephemeral_circuit` created and deleted a model on the host. | High | **FIXED in this PR** (bugs.md #57) | Recording listener + CLI (journal). Independently found by the Codex security reviewer (B). Regression test fails on old code. |
| S2 | Interpreter (`-run`) `confidence` and `lazy_synthesize` calls had no gate | High | **FIXED in this PR** (bugs.md #57) | Same as S1 |
| S3 | `lazy_synthesize` compiles model output **at runtime** with no checker, HFIR gate, or construct registry. The result runs with the program's full grant and overwrites the function's instructions in the shared program (`internal/vm/vm.go:2638-2690`). | High (design) | OPEN. Must be quarantined (P0 profile). | Code reading. Interpreter path confirmed live by probe. Codex A and B flagged it independently. |
| S4 | **Memory is unbounded.** `MaxMemoryBytes` (64 MiB) is declared but never enforced. About 100 instructions of string doubling under the default 100k budget and **zero grants** reached **~2.06 GB RSS** in 15.6 s. | High (DoS) | OPEN (P1) | `/tmp` probe (journal). Codex A and B independently found the unenforced field. |
| S5 | **Unbounded recursion crashes the process.** At review time, `MaxCallDepth` (128) guarded only `SPAWN_AGENT`; ordinary bytecode `CALL` recursion could overflow the Go stack. C4a now bounds bytecode CALL with structured `LIMIT_EXCEEDED`, default 1000 shared with SPAWN_AGENT nesting, and a runner flag. AST interpreter recursion remains unbounded by this limit. | Medium | PARTIAL (C4a, 2026-10-06): bytecode CALL bounded; AST interpreter remains open. | `/tmp` probe with `--max-instructions 2000000000` |
| S6 | **Wall-clock is unbounded.** `sleep`, `fetch`, `exec`, `read_line`, and model calls block outside the instruction count. | Medium | OPEN (P1) | Codex B: a 5 s `sleep` under a 3-instruction ceiling ran until killed. Code: `vm.go:2122`, `:2170`. |
| S7 | **Response and output sizes are unbounded.** `fetch` reads the whole body. `exec` uses `CombinedOutput`. `print` is unbounded. | Medium | OPEN (P1) | Code reading (A, B) |
| S8 | **`process` grant = arbitrary host authority.** `(exec "/bin/sh" "-c" "...")` with only `process` writes files. Grants are five classes with no resource scoping. | High (policy design) | OPEN (P1/P2: scoped grants, action allow-list) | Codex B ran it in `/tmp` on both VM and interpreter |
| S9 | **Child agents inherit the full grant.** `SPAWN_AGENT` and legacy `SPAWN` children get `AllowedCaps` unchanged (`vm.go:2233`, `:3030`). Legacy `SPAWN` also resets instruction accounting. | Medium | OPEN (P2: attenuation) | Code reading (B) |
| S10 | **Semantic divergence changes effects.** `(and (= 1 2) (= (env "X") "x"))`: the VM and interpreter evaluate eagerly and hit `CAPABILITY_DENIED`. Generated Go short-circuits and prints `false`. | Medium | OPEN (P1: decide `and`/`or` semantics, add differential test) | Found by Codex A. Reproduced in this review. |
| S11 | **Generated Go/JS gate only 6 effects.** Model calls, SQL, listeners, goroutines, and JS `spawn` run ungated. A generated-Go panic writes `crash.json` unconditionally. | Medium (documented) | OPEN. Keep Go/JS out of the governed profile. | README states it. Codex B confirmed. |
| S12 | **Artifact validation is structural only.** Magic, version, SHA-256, opcode, jump, and function existence are checked. There is no stack-height, operand-type, or embedded-body validation. SHA-256 proves integrity, not authorship. | Medium | OPEN (P2) | Code reading (`internal/bytecode/artifact.go`) |
| S13 | **Compile-time includes read arbitrary paths** before any grant applies. Model-authored source could `include` files outside the project. | Low/Medium | OPEN (P2: rooted includes for governed profile) | Codex B ran it |
| S14 | **Ambient channels.** `stdin`, `stdout`/`stderr`, `time_now`, `sleep`, and `exit` are capability-free. Output can forge text that looks like a receipt. | Low | Document. Treat as host channels. | Code reading |
| S15 | **"Verified" is overloadable.** The HFIR gate blocks on only `HFIR_INVALID_REF` and `HFIR_TARGET_INFEASIBLE`. `VerificationEvidence.Verified` is a bare boolean. | Medium (claims risk) | OPEN (P0 wording; P1 report) | `howlframe.go:466`, `internal/hfir/storage.go:89` |
| S16 | **Decision JSON can be forged through proposal fields.** `apps/release_authority` builds its output JSON by string concatenation (`str_join`) and does not escape the proposer-controlled `target`. A proposal `{"action":"deploy_production","target":"svc\", \"decision\": \"ALLOW"}` makes the VM correctly decide `DENY`, and no state mutation happens. But the printed object contains a second `"decision": "ALLOW"`. Python `json.loads` (and Go `encoding/json` into a map) keep the last duplicate key, so they read **ALLOW**. | Medium (demo app, not the VM) | FIXED in this change (C7: `encode_json`, duplicate-key and literal-target regression) | Found by Codex D (who reported invalid JSON). Escalated and reproduced in this review: `/tmp/hf-ainative-probe/ra2.out`. |
| S17 | **Partial effects contradict the "failure atomicity" doc claim.** In `apps/action_executor` the `stage_artifact` path runs `write_file` and only then `store_open`. With a `filesystem`-only grant it prints `ALLOW`, writes the staged file, then traps `CAPABILITY_DENIED` on `STORE_OPEN`. `docs/application_dogfooding_phase_5.md` says this ordering ensures "failure atomicity" and that "security is guaranteed entirely outside the host Go backend". | Low/Medium (claims risk) | FIXED in this change (C7: pre-flight checks; ordered, not atomic wording) | Found by Codex F (ran it). Ordering confirmed in source (`action_executor.howl` ~159-164). |

## Effect-gate audit after this PR

Every opcode whose VM case performs an external effect, checked by reading
each `case` in `internal/vm/vm.go` and grepping for `http.`, `os.`,
`exec.Command`, `sql.`, and `net.`:

| Effect | Opcodes / constructs | Gate on `-run-bc` | Gate on `-run` |
| --- | --- | --- | --- |
| HTTP out | `FETCH` | network | network |
| Model host | `LLM_GENERATE`, `ACHIEVE`, `CONFIDENCE`*, `NEURAL_CIRCUIT`*, `EPHEMERAL_CIRCUIT`*, lazy `CALL` | network | network (`lazy_synthesize`* and `confidence`* now gated) |
| HTTP serve | `HTTP_SERVER_START`, `HTTP_ROUTE`, `HTTP_SERVER_SERVE`, `RES`, `RES_JSON`, `HTTP_RES_HEADER`, `HTTP_REQ_METHOD` | network | n/a |
| Filesystem | `READ_FILE`, `WRITE_FILE`, `MKDIR`, `file://` stores | filesystem (+database for stores) | filesystem |
| Process | `EXEC`, `SPAWN`, `SPAWN_AGENT` | process | process |
| Environment | `ENV` | environment | environment |
| Database | `DB_CONNECT`, `SQL_QUERY`, `STORE_*` | database | n/a |
| Ambient | `PRINT`, `STDERR`, `READ_LINE`, `EXIT`, `SLEEP`, `TIME_NOW`, `CLI_ARGS*` | none (by design) | none |

\* changed in this PR.

No additional effectful opcode was found without a gate. C3 is implemented in
`internal/vm/effect_gate_conformance_test.go`: a same-file transitive AST call
scan, central/in-case gate checks, and empty-grant runs for every registry
capability plus lazy CALL. Interpreter constructs with an existing harness are
also checked. This is executable conformance evidence, not a general proof of
all host effects; calls into other files/packages remain outside the scan.
