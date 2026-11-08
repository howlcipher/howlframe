# AI-DX1 understanding interface

## Scope and routing

Starting revision: `44821dc8a08f8942889a864ed25ac9a2e186383d`.
Implement one bytecode-first understanding slice: `agent capabilities`, `agent describe`, `agent check --json`, and `agent explain`. Existing compilation, execution, grant enforcement and HFIR verification remain authoritative. Other known backend targets report unknown registry coverage; unrecognized targets fail closed. No M6, snapshot or repair implementation is included.

The control-plane task specification is AI-DX1, medium risk, tier 2. The router recommended agy and correctness, test-falsifier, regression and simplicity review. Execution uses available Codex delegation instead: no additional provider, paid call or unverified external execution route. The canonical control-plane CLI required `PYTHONPATH=src` in HowlPlane; its compatibility invocation without that environment failed without changing files. Work is isolated in the `feat/agent-understanding` worktree. Other live Claude/agy sessions have different working directories; no AI-DX worktree or journal was found.

## Architecture decision

Reuse construct support and arity registries, existing diagnostic formats and the production bytecode compilation gate. Benefits: no copied support database, consistent fail-closed checks and executable evidence. Limits: arity is not a complete typed signature, support is target-specific, and the legacy compiler exits on diagnostics. A separate compiler/checker refactor would avoid subprocess normalization but increases shared compiler risk and scope; a handwritten metadata catalog is easier initially but drifts. Keep unknown metadata explicit, and isolate legacy diagnostic normalization behind the new interface.

## Preregistered measurement protocol

Frozen before implementation: compare the same five task classes on this starting revision and the final interface revision: discover print signature; discover read_file authority; check a valid pure CLI; diagnose invalid append arity; reject unsupported bytecode test blocks. Use the exact fixture bytes and SHA-256 hashes in [the preregistration](../reference/ai_dx1_measurement_protocol.md). Deterministic fixtures establish correctness only; they do not demonstrate token or time savings.

Live-agent comparisons remain deferred until a verified no-payment route is available. Before those runs, freeze exact fixture bytes/hashes, dependencies and tool versions, equal model/tool budgets, randomized matched ordering and held-out behavioral checks. Include interruption after edits, during validation and after evidence recording. Record failed and interrupted attempts, total context, repair, snapshot, verification and elapsed costs, commands and retries; report paired results and dispersion for AI-DX1 separately. Do not claim experimental reduction targets as achieved. No new model calls are authorized by this protocol.

## Test impact assessment

Observable change: new read-only machine interfaces expose target-specific support, argument counts, actual grants (none for inspection), limitations, normalized diagnostics and verified example references. Existing CLI commands and compiler semantics must remain unchanged. Existing construct/arity tests protect registry consistency, HFIR tests protect target feasibility and root CLI tests protect command behavior; these contracts remain intended.

Missing coverage to add: deterministic complete registry output; unknown versus unsupported target; malformed arguments; unknown constructs/codes; parser/type/arity/HFIR failures and stable locations; no program execution or output artifacts; capability requirements distinct from actual grants; advertised examples compiled/executed under empty grants. Use affected unit/CLI tests first, relevant registry/diagnostic tests next, then full Go test, vet and build before integration. Heavy checks run sequentially with the recorded host and scope limits. No test removals or weakened contracts are planned.

## Evidence and review

The initial unit and CLI checks passed, followed by the final affected gate including registry/arity, agent CLI and interop tests. The affected gate peaked at 157,560,832 sampled bytes and 24 tasks, with zero memory/pids events. Advertised example records are actually compiled and executed without grants; pure and effect-requiring checks are repeated byte-for-byte. No tests were removed. Full `go test ./... -count=1` passed, as did `go vet ./...`, `go build ./...`, `go mod tidy` with unchanged dependency files, formatting, SEO and local documentation links. The generic control-plane runner first timed out its full Go step at 300 seconds; the fresh full run passed in the recorded 2,400-second bounded scope. The root CLI package took 302.587 seconds. Peak sampled memory was 830,873,600 bytes with 61 tasks; memory and pids events were zero.

Independent test/security/regression review caught and verified corrections for silently dropped compiler warnings, fabricated filenames on legacy imported diagnostics, insufficient scoped exec authority metadata, interop discovery omission and an example test that could miss advertised-record drift. Final independent correctness/simplicity review found no integration blocker. Two low-severity documentation mismatches were corrected: compiler launch errors use CHECK_FAILED, and target values are consumed literally before validation. No finding was silently dismissed.

Timeout, output-limit, launch-error and artifact-read fault injection remain test coverage gaps; current production-path tests cover ordinary success and parse, arity, type and HFIR rejection, warnings, imported failures, no execution, temporary cleanup and authority inventories. These unexercised exceptional branches are not claimed as verified. Full typed signatures and non-bytecode catalogs remain outside this first slice.

Durable local task specification, routing, source hashes, review reconciliation and logs are under `/run/media/howlcipher/tallgeese/dev/worktrees/hf-agent-evidence`. The full-run log is `full-tests.log`; the original failed generic plan remains `verification.json` and is not overwritten as a passing result.

## Inherited hygiene gate

The generic control-plane run failed Slopslint enforcement: 745 production clones against ceiling 88. Repeating the same command against clean starting main produced exactly the same failure/count. Neither result is passing. This is filed as bug #72; no ceiling or exclusion was changed. The project's public CI has no hygiene step. The previous owner exception concerned private consumer hosted CI only and is not treated as a waiver for this failure. The owner subsequently authorized continuing with the documented inherited hygiene exception. Bug #72 remains open; the enforcement failure is not reclassified as passing.

Diagnostic classification scans completed for both scopes on baseline and candidate. Active clone counts are unchanged: production 745 (ceiling 88), tests 644 (ceiling 135). Production duplicated lines changed from 12,818 to 12,826 within the same clone count; test duplicated lines remain 10,366. Candidate scans include the added production file and two test files (348 production sources and 300 test sources versus 347/298 on baseline). These scans provide comparison evidence, not an enforcement pass. Raw reports are retained as `baseline-classification.json` and `candidate-classification.json`.

## Accepted disposition

On 2026-11-08, the owner authorized proceeding in response to the explicit request to merge PR #305 with this documented hygiene exception. The initial AI-DX1 bytecode slice is complete with that exception; broader target/signature coverage and exceptional-path fault injection remain deferred as recorded in the backlog.

Implementation commit `9d3c32d4c3fb178bbb9a730ed12d9c3766f226b6` passed public Build, Acceptance, Race and Latest Go in [CI run 42501975334](https://github.com/howlcipher/howlframe/actions/runs/42501975334). This closeout updates documentation only. Source hashes match the locally tested implementation; no source, tests, dependencies, test configuration or hygiene policy changed. Documentation links, SEO and whitespace are checked before the signed closeout commit; the updated PR must pass all four public checks before merging.

This exception applies to the inherited hygiene failure for this integration, not to future regressions or a higher clone ceiling. No efficiency gain, deployment, paid execution or private hosted run is claimed.
