# Repo Analyst dogfooding session

## Objective

Build the deterministic Zero Repo Analyst reference application primarily in Zero. Use the application to discover the first meaningful limitation, classify it, implement at most one justified general capability, prove it in the application, and stop at the next blocker.

## Preflight

Improvement #95 is fully Done on `main` at commit `052d56a`. The tracker and `docs/journals/2026-08-08_improvement_95_standalone_modules.md` agree. Focused `TestModule*` tests and the parser, checker, construct, ZIR, bytecode, and VM package tests passed. A scratch compiler built from the current tree compiled `tests/module_main.zero` to bytecode and the VM produced `42` and `20`.

## Application location

The blueprint defines reference applications but does not prescribe a separate top-level directory. Existing user-facing programs live under `examples/`, so this application uses `examples/repo_analyst/`. A subdirectory is necessary because Improvement #95 needs to be exercised by a real multi-file application.

## Limitation log

### Recursive repository discovery

- Task being attempted: accept a repository path and recursively discover regular files while excluding `.git` internals.
- Code written: `discovery.zero` invokes the existing `(exec ...)` primitive with `find`, converts its byte output to a string, and splits newline-delimited paths into a Zero list.
- Initial classification: no Zero change justified. Existing primitives appear composable.
- Capability effect: this design requires `process` for `exec`. Later file inspection will also require `filesystem`.
- Portability note: relying on the host `find` executable is less portable than a Zero-owned directory API, but portability alone is not yet a proven blocker. The application must first test the existing path.
- Tracker relationship: Improvement #33 historically mentioned `list_dir`, but its Done note records only `write_file` and `mkdir`. No tracker change is warranted unless the existing composition fails or proves inadequate for the application.

### `stderr` arity during discovery error handling

- Task being attempted: report a failed host discovery command with context.
- Observed failure: validation returned `{"reason":"stderr expects 1 argument, got 2","line":9,"column":11}`.
- Classification: application design problem.
- Decision: compose the message with `str_join` and pass one string to `stderr`. No tracker item or Zero change is justified.

### Initial aggregation parse failure

- Task being attempted: add classification, text inspection, aggregation, and report output using ordinary Zero modules.
- Observed failure: validation returned `{"reason":"Expected ')'","line":110,"column":1}`; the application root had one unmatched opening parenthesis.
- Classification: application authoring error.
- Follow-up diagnostic: after adding the missing final delimiter, validation returned `{"reason":"for expects 3 arguments, got 4","line":31,"column":19}`. The delimiter belonged at the end of the loop body rather than the file end, so the report-building `let` had accidentally become a fourth `for` argument.
- Decision: move the delimiter to close the `for` body at its semantic boundary and re-run validation. These localized diagnostics made the authoring mistakes repairable; neither is evidence for a language change.

## First meaningful blocker

**Observed application need:** Read every discovered text file inside a `for`, recover from an individual `read_file` failure with `try_let`, and continue deterministic aggregation.

**Current Zero behavior:** The complete multi-module source validates and compiles to bytecode. Running it through the standalone VM processes the first file, then fails at the enclosing loop with `{"phase":"runtime","code":"STACK_UNDERFLOW","function":"main","instruction":52,"opcode":"FOR_NEXT","message":"stack underflow at FOR_NEXT"}`.

Three reductions separated the interacting features:

- An outer `for` that calls a function succeeds.
- An outer `for` that calls a function containing another `for` succeeds.
- An outer `for` containing `try_let` fails after its first iteration. Before the failure, it unexpectedly prints the outer iterator's file list and index, proving that a repeated branch instruction consumed the iterator state.

Inspection confirmed the control-flow defect. `OpTryLet` embeds its value, catch, and success instructions immediately after the opcode. After executing the selected branch, the VM uses `ip += valLen + catchLen + successLen`. The correct next instruction is one position farther: `oldIP + 1 + valLen + catchLen + successLen`. The current jump lands on the final embedded branch instruction and runs it twice. In this application that instruction is `PRINT`; its second execution pops the enclosing `for`'s retained list and index, so the next `FOR_NEXT` underflows.

**Classification:** BUG.

**Can existing primitives compose into a solution?** Yes. `for`, `try_let`, and `read_file` already express the application need. A source-level early `return` can incidentally bypass the bad post-branch jump in some shapes, but that is not a valid general workaround and would not preserve the required aggregation flow.

**Should this capability exist outside Repo Analyst?** The existing composition must work for any program. A backup utility, batch data converter, and log processor all plausibly loop over independently fallible operations.

**Smallest reusable abstraction:** No new abstraction. Repair the existing `OpTryLet` instruction-pointer contract so each branch executes once and control resumes after all embedded instructions.

**Does it require:**

- checker change? No.
- ZIR change? No.
- verifier change? No.
- bytecode change? No opcode or format change.
- VM change? Yes, one instruction-pointer correction.
- capability change? No.
- docs? The authoritative bug tracker and this dogfooding journal.
- tests? Focused VM regressions for both selected branches, plus application-level standalone bytecode proof.

**Existing tracker item:** No existing bug or improvement describes this instruction-pointer failure. Added narrowly scoped bug #46 to `bugs.md`; bug #8 concerns historical error-source generalization and is already Done, while improvement #84 concerns Wasm lowering and is unrelated.

**Recommended action:** First add focused regressions that fail on both success and catch paths inside a multi-iteration `for`. Then change only the VM's post-`try_let` instruction advance, rerun the focused tests, and return directly to Repo Analyst. Do not add syntax, opcodes, ZIR nodes, capabilities, or libraries.

## Bug #46 resolution

- Pre-fix regression evidence: both the success and catch cases in `TestBytecodeTryLetResumesAfterEmbeddedInstructionsInFor` failed with `STACK_UNDERFLOW` at `FOR_NEXT`. Their stdout was respectively `"one\n1\n"` and `"caught one\n1\n"`, demonstrating the duplicated final `PRINT` and leaked outer-loop index.
- Production change: `internal/vm/vm.go` now advances past the `OpTryLet` instruction as well as all three embedded instruction regions: `1 + valLen + catchLen + successLen`.
- Semantic effect: the selected branch runs exactly once and the outer dispatcher resumes at the first instruction after `try_let`. No checker, ZIR, verifier, bytecode representation, opcode, capability, or source-language behavior changed.
- Focused proof: both success and catch regressions now pass using checker-valid source; the catch case performs a genuinely failing `read_file` with the filesystem capability. The complete parser, AST, checker, construct-registry, ZIR, bytecode, and VM package sweep passes.
- Direct-interpreter boundary: the Phase 1 AST interpreter does not implement `try_let` or `read_file`. Adding those constructs would be a separate backend feature, so this VM-only repair uses the full existing difftest as its cross-backend regression gate rather than expanding scope for a new parity oracle.

## Application proof after bug #46

The application now has five Zero source modules: CLI/aggregation, discovery, classification, text analysis, and report rendering. It compiles only to Zero bytecode and runs in the standalone VM with exactly `process,filesystem` granted. No AI primitive is present.

The end-to-end test copies all five application modules into a temporary source directory, invokes `-compile-bc`, deletes that entire source copy, and only then invokes `-run-bc`. This proves that the artifact does not load imported `.zero` files at runtime. It creates an eight-file repository containing Zero, Go, Python, JavaScript, TypeScript, configuration, test, entry-point, and non-text cases. The report correctly returns eight total files, the expected per-language counts, one configuration file, one test, two likely entry points, seven text files, two `TODO` markers, one `FIXME`, and the exact largest text file and byte count. A repeated run is byte-identical. The test then adds an equal-size text file and proves that the report retains the maximum byte count while leaving the path empty, so filesystem enumeration order cannot choose the winner. Optional output-file mode, missing-argument exit 2, discovery-failure exit 3, and fail-closed process-capability denial are also exercised. It does not generate Go or JavaScript application code.

## Next blocker — not implemented

**Task being attempted:** Analyze Repo Analyst's own directory after the focused fixture succeeded.

**Code exercised:** The same unchanged application and bytecode artifact, pointed at `examples/repo_analyst/`. The directory contains seven files totaling 20,149 bytes.

**Observed failure:** The standalone VM returns `{"phase":"runtime","code":"LIMIT_EXCEEDED","function":"main","instruction":7,"opcode":"BINOP","message":"instruction limit exceeded"}` before producing a report.

**Classification:** TOOLING GAP. The Zero program is expressible and the VM already has an instruction-limit mechanism, but the standalone runner has no host-controlled way to select that existing policy for a legitimate workload.

**Current Zero behavior:** `RunBytecode` unconditionally installs `DefaultLimits`, including `MaxInstructions: 100000`. The `-run-bc` CLI exposes capabilities but no runner-selected execution budget. Repo Analyst's ordinary Zero `byte_count` loop uses several VM instructions per byte, so even this small directory exceeds the fixed ceiling.

**Can existing primitives compose into a solution?** They compose correctly for small inputs, as the application test proves. Repo Analyst could outsource byte counting to host `wc`, but that would be a platform-dependent application workaround and would not solve aggregate instruction growth across many files. Zero source cannot raise the VM limit, and it should not be allowed to self-grant runtime policy.

**Should this capability exist outside Repo Analyst?** Yes. Backup tools, static-site generators, and batch converters all need a trusted runner to select a bounded budget appropriate to caller-controlled workload size.

**Smallest reusable abstraction:** A validated runner-controlled instruction budget threaded into the existing `VMLimits` mechanism, preserving the current default and `LIMIT_EXCEEDED` behavior. This is not a source-language primitive.

**Does it require:**

- checker change? No.
- ZIR change? No.
- verifier change? No.
- bytecode change? No.
- VM change? A public limits-aware entry point or equivalent parameter threading.
- capability change? No; runtime resource policy is not a source-grantable capability.
- tooling change? Yes, a validated `-run-bc` policy option.
- docs and tests? Yes, including exact-boundary, invalid-value, and Repo Analyst scaling proofs.

**Tracker decision:** None matched configurable standalone instruction budgets. No second numbered item was created because this session's one tracker addition is Bug #46, which owns the implemented Zero repair. The next blocker is recorded here for a future session. Improvement #98 concerns durable artifact envelopes and must remain separate; improvement #96 concerns native test blocks and is a later application limitation.

**Recommended action:** Stop this session without changing the budget. In a separate session, create a narrowly scoped tracker only after rechecking the backlog, then use `gpt-5.6-sol` in `plan-then-reviewed-edit` mode because runner-controlled safety limits require an explicit boundary review. A generic collection-length operation may later earn separate consideration, but it is not required for the runner-policy fix.

## Verification

- Preflight: `go test . -run '^TestModule' -count=1`; focused parser/checker/construct/ZIR/bytecode/VM package sweep; scratch module compile and standalone VM run produced `42` and `20`.
- Bug regression before fix: both `try_let` branch cases failed with `STACK_UNDERFLOW` and leaked iterator output.
- Bug regression after fix: `go test ./internal/vm -run '^TestBytecodeTryLetResumesAfterEmbeddedInstructionsInFor$' -count=1` passed.
- Focused packages: `go test ./internal/bytecode ./internal/vm` passed.
- Application: `go test ./examples/repo_analyst -run '^TestRepoAnalystStandaloneBytecode$' -count=1 -v` passed.
- Full suite: `go test ./...` passed.
- Static analysis: `go vet ./...` passed.
- Final compiler/VM build: `go build -o /tmp/zero-dogfood-final zero.go` passed.
- Differential gate: `go run tools/difftest/main.go` completed with 17 passed, 27 skipped, and 0 failed.
- Formatting/worktree gates: `gofmt -l .` and `git diff --check` passed; final `git status --short` is recorded in the handoff.
- Benchmark gate: not run. The production diff only corrects the bytecode VM's post-`try_let` instruction pointer and does not change any benchmark-named source-writing surface or emitted program.

## Session outcome

Bug #46 is Done. Repo Analyst is a useful deterministic standalone-bytecode MVP on bounded repositories and is the application-level proof for the repaired `for` plus `try_let` composition. Configurable runner-controlled instruction budgets are the next blocker and remain unimplemented. No AI functionality was added, no second tracker item or Zero capability was implemented, and no generated Go or JavaScript application path was used.
