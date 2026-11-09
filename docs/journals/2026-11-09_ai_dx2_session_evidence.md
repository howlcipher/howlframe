# AI-DX2 session evidence implementation

Status: locally verified candidate, 2026-11-09; public CI and integration pending. Baseline compiler `358fe92acf320460af022ea62d10644fb968f342`. This is the next ordered roadmap item after merged M6; no AI-DX3 work or efficiency gain is claimed.

## Scope and decisions

Implement local versioned snapshots, task-scoped context and verified status. Use a narrow compiler-check producer, not a general command runner. This reuses the production compiler and avoids introducing execution authority for source programs. It cannot record arbitrary application test runs; those remain outside this initial slice. Claims and remaining actions are owner/agent statements, separate from observed compiler results.

Local authenticated bundles are inspectable and have no service cost. Conservative source/environment/compiler invalidation costs repeated checks. A database adds indexing/concurrency but also operational and authority complexity; it is deferred. A portable unsigned report is simpler but permits edited pass claims, so local trust remains separate from exported evidence. Same-user access to the trust material is outside authentication guarantees.

The [measurement protocol](../reference/ai_dx2_measurement_protocol.md) was frozen before implementation. Deterministic tests establish correctness only; live-agent comparisons remain deferred pending a verified no-payment route.

## Test impact assessment

Existing understanding unit/CLI tests protect the bytecode discovery envelope, diagnostics, target boundaries and non-execution behavior; these contracts remain intended. New tests must protect snapshot creation/recovery, exact task identity, dirty/untracked and dependency/config/test changes, altered/missing evidence, compiler/environment changes, interrupted publication and trust/path rejection. No existing tests are scheduled for removal. Run directly affected tests, then agent/CLI integration, then full Go build/vet/test and required public Build, Acceptance, Race and Latest Go checks. Final results must match the final source/test/config tree.

## Verification and integration

The focused internal/agent and root agent/interop gate passed (2.651s and 5.433s). Full Go vet, build and uncached tests passed: 12.323s, 18.777s and 1054.373s. Formatting passed and `go mod tidy` left dependency files unchanged. Local Go was 1.26.0; Node 22.22.1, wasmtime 47.0.3, wasm-tools 1.254.0 and system Git 2.53.0 were available.

The final source/test/configuration inventory contains 906 hashed files and remains unchanged. Heavy checks ran sequentially in fresh scopes: MemoryHigh 1 GiB, Max 2 GiB, SwapMax 0, CPU 100%, TasksMax 128, GOMAXPROCS 2, package parallelism 1, GOMEMLIMIT 384 MiB, 8 GiB launch headroom and 7 GiB runtime reserve. This is a smaller Go-only workload profile, not the M6 Chromium exception. Full-run kernel memory peak was 1,012,965,376 bytes, task peak 64, minimum host available memory 8,495,034,368 bytes, scope swap zero and memory/pids events unchanged at zero. Maximum host PSI some/full avg10 was 0.54/0.54. Host swap counters changed and are retained; they are not attributed to this scope. No browser engine was launched by this task.

Independent five-role review caught two confirmed defects: ambient PATH could substitute Git, and the external monitor could miss resource events during descendant cleanup. Trusted absolute Git lookup, a hostile-PATH regression, post-cleanup final counters and two mocked monitor regressions corrected them. Re-review found no outstanding findings. Real failing compiler and observed child-environment tests supplement checker-mock unit tests. Original findings and remediation remain in the evidence; none was silently dismissed.

The control-plane verification plan uses a task-local 1800-second full-test timeout and `-count=1`, without changing repository policy. Its overall result is **failed**, because Slopslint enforcement still exits 1: 745 production clones against ceiling 88. Baseline and candidate classification have identical active counts (745 production, 644 test) and duplicated lines (12,826 production, 10,366 test). No ceilings, exclusions or tests changed. This is inherited bug #72, not a passing gate. The AI-DX1 exception does not automatically apply to this integration. Explicit owner acceptance and successful public Build, Acceptance, Race and Latest Go checks are required before merging.

Missing coverage: exhaustive filesystem fault and size-limit injection, and adversarial same-UID races. Unsupported dependency sources, installed dependencies, arbitrary command/test producers and excerpt selection remain outside the initial slice, as documented in the interface and follow-up backlog. No efficiency gains or broader workflow verification are claimed.

TaskSpec, TIA, source inventory, review/reconciliation, resource events and original logs are retained under `/run/media/howlcipher/tallgeese/dev/worktrees/ai-dx2-evidence`. Evidence index SHA-256: `635a19a0c877229c2ef9da46b53ab1be2576c602b099938175e7e44cc175782a`. Final public CI and integration evidence must be recorded before completion.
