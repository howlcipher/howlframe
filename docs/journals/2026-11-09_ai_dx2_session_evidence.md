# AI-DX2 session evidence implementation

Status: initial compiler-check slice implemented and locally verified, 2026-11-09. [PR #307](https://github.com/howlcipher/howlframe/pull/307) records final integration status, public checks and the owner decision on inherited hygiene debt. Baseline compiler `358fe92acf320460af022ea62d10644fb968f342`. This is the next ordered roadmap item after merged M6; no AI-DX3 work or efficiency gain is claimed.

## Scope and decisions

Implement local versioned snapshots, task-scoped context and verified status. Use a narrow compiler-check producer, not a general command runner. This reuses the production compiler and avoids introducing execution authority for source programs. It cannot record arbitrary application test runs; those remain outside this initial slice. Claims and remaining actions are owner/agent statements, separate from observed compiler results.

Local authenticated bundles are inspectable and have no service cost. Conservative source/environment/compiler invalidation costs repeated checks. A database adds indexing/concurrency but also operational and authority complexity; it is deferred. A portable unsigned report is simpler but permits edited pass claims, so local trust remains separate from exported evidence. Same-user access to the trust material is outside authentication guarantees.

HMAC-SHA256 keeps the single-owner local trust implementation small using the standard library; every key holder can authenticate bundles. Ed25519 would allow public-key-only verifiers but requires a signer/key-distribution policy to establish whose execution is trusted. That separate-role or remote-attestation claim is outside this slice.

The [measurement protocol](../reference/ai_dx2_measurement_protocol.md) was frozen before implementation. Deterministic tests establish correctness only; live-agent comparisons remain deferred pending a verified no-payment route.

## Test impact assessment

Existing understanding unit/CLI tests protect the bytecode discovery envelope, diagnostics, target boundaries and non-execution behavior; these contracts remain intended. New tests must protect snapshot creation/recovery, exact task identity, dirty/untracked and dependency/config/test changes, altered/missing evidence, compiler/environment changes, interrupted publication and trust/path rejection. No existing tests are scheduled for removal. Run directly affected tests, then agent/CLI integration, then full Go build/vet/test and required public Build, Acceptance, Race and Latest Go checks. Final results must match the final source/test/config tree.

## Final corrected local verification

The final affected internal/agent and root agent/interop gate passed (2.944s and 5.250s). Final Go vet, build and uncached full tests passed: 1.830s, 13.564s and 962.844s. Formatting passed and `go mod tidy` left dependency files unchanged. Local Go was 1.26.0; Node 22.22.1, wasmtime 47.0.3, wasm-tools 1.254.0 and system Git 2.53.0 were available. The 906-file `final-source-v2.json` source/test/configuration inventory matches the corrected candidate; documentation-only closeout does not change those inputs.

Heavy checks ran sequentially in fresh scopes: MemoryHigh 1 GiB, Max 2 GiB, SwapMax 0, CPU 100%, TasksMax 128, GOMAXPROCS 2, package parallelism 1, GOMEMLIMIT 384 MiB, 8 GiB launch headroom and 7 GiB runtime reserve. This is a smaller Go-only workload profile, not the M6 Chromium exception. The corrected full-run kernel memory peak was 843,874,304 bytes, task peak 64, minimum host available memory 8,391,290,880 bytes, scope swap zero and memory/pids events unchanged at zero. Maximum host PSI some/full avg10 was 1.10/0.94, below the 10/1 stop thresholds. Host swap counters changed and are retained, without attributing them to this scope. No browser engine was launched by this task.

Independent five-role review and follow-up re-review closed three confirmed defects:

1. DX2-R1, high: ambient PATH could substitute Git. Trusted absolute Git lookup and a hostile-PATH regression corrected it.
2. DX2-R2, medium: the external monitor could miss events during descendant cleanup. Post-cleanup final counters and two mocked regressions corrected it.
3. DX2-R3, medium: malformed UTF-8 identities could produce initially verified but unrecoverable snapshots; malformed environment values could alias in the hash preimage. Red regressions reproduced both defects. Text options, inventory names and paths now reject invalid UTF-8, including resolved relative paths; non-source contents and opaque environment values retain raw-byte hashing. Green regressions and independent re-review close the condition.

No finding was silently dismissed. Real failing compiler and observed child-environment tests supplement checker-mock unit tests. Missing exhaustive filesystem fault and size-limit injection, adversarial same-UID races, unsupported dependency sources, installed dependencies, arbitrary test producers and excerpt selection remain explicit limits. No efficiency gains or broader workflow verification are claimed.

## Hygiene and integration authority

The task-local control-plane plan uses an 1800-second uncached full-test timeout without changing repository policy. Its overall result remains **failed** solely because Slopslint enforcement exits 1: 745 production clones against ceiling 88. Baseline and corrected candidate have identical active counts (745 production, 644 test) and duplicated lines (12,826 production, 10,366 test). No ceiling, exclusion or test was weakened. Bug #72 remains open; this failed gate is not relabeled as passing.

The prior owner exception covered AI-DX1. Integrating this PR requires its own explicit owner acceptance of the unchanged inherited hygiene failure, recorded in PR #307, plus successful public Build, Acceptance, Race and Latest Go checks on the corrected final head. The merged-main checks must also be verified before task completion. The implementation is an initial compiler-check slice; deferred coverage is recorded in the backlog and interface contract.

## Revision-bound evidence

The first signed implementation was `59451d712d11e33816f7dcf277a7f63d2a4938e2`. Its local Go gates passed and all four public checks passed in [run 42718780339](https://github.com/howlcipher/howlframe/actions/runs/42718780339). DX2-R3 was discovered afterwards, so those results are retained as original-candidate evidence, not corrected-tree proof. Original evidence index and artifacts remain byte-exact.

Final corrected local results are `affected-v2.log`, `verification-v2.json` and `full-verify-v2.log`. The final review is `independent-findings-v2.json`; reconciliation, TIA, original failures and resource events remain under `/run/media/howlcipher/tallgeese/dev/worktrees/ai-dx2-evidence`. Corrected evidence-index SHA-256: `8cdbb8d0cd4ae19b993d26c325c3ee88604319ad785edc628689f9f570e838c2`. Final PR/main outcomes and signatures are recorded in GitHub and the task ledger, with the final merge subject using Conventional Commits. No paid execution, private hosted run, deployment or new consumer compatibility claim is included.
