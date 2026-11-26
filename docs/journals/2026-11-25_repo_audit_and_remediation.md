# HowlFrame Repository Audit, Roadmap Assessment & Remediation Journal

**Date:** 2026-11-25
**Branch:** `fix/audit-remediation` (from `main` at `187bc073`)
**Scope:** Approved Option B remediation pass (Hygiene + CLI/Test/Script/Doc Fixes + Re-entrant Backends + `#111` Persistent `append` + `#72` `slopslint` Gate Restoration + `P1.3` Verification Report v0)

---

## 1. Baseline Verification on Clean `main` (`187bc073`)

All baseline checks were executed directly against clean `main` (`187bc073`) before making edits:

| Check | Outcome | Notes |
| --- | --- | --- |
| `gofmt -l .` | PASS | Clean formatting |
| `go vet ./...` | PASS | Clean across all Go packages |
| `cmd/codegen` drift check | PASS | `bytecode_reference.md`, `construct_coverage.md`, `orchestrator_schema.py` match generator output |
| `slopslint check --classify --enforce` | FAIL | `746` active `go_production` clones across `358` sources vs ceiling `88` (`experiments` `541`/217, `benchmarks` `41`/33, `internal` `145`/101 [`internal/vm` `44`/15/`3003` dup lines, `internal/backend` `30`/23/`445`, `internal/hfir` `15`/16/`214`], `tools` `3`/2, `cmd` `0`/2; plus `647` `go_tests` clones) |
| `HOWL_FORBID_LOCAL_INFERENCE=1 go test -p 1 -count=1 ./...` | FAIL (`apps/echo_api`) | Every package passed (`root` `257.223s`, `wasmgc` `198.380s`, `difftest` `68.836s`, `javascript` `62.980s`, `gogen` `31.768s`, `vm` `13.769s`) except `apps/echo_api` (`127.0.0.1:8106 is already in use`, held by pre-existing orphan PID `2206280` whose deleted cwd was an old worktree `/tmp/.../wt-021/apps/echo_api`) |
| `bash scripts/ci_acceptance.sh` | PASS | Ledgerlens `35/35`, Healthwatch `29/29`, Taskflow `32/32`, Ticketdesk `101/101` + `13/13` DX + `11/11` reassessment + `10/10` historical + `5` m10 (`1` waived `XFAIL m10_port_lost_race_stderr_shape`) + `3/3` restart/kill |
| `pytest -q tests/test_observer_agent.py` | PASS | `9 passed in 0.59s` |
| `python3 scripts/test_seo.py` | PASS | All SEO validations passed |
| `python3 -m unittest discover -s benchmarks/v2/harness -p 'test_*.py'` | PASS | `Ran 8 tests in 13.254s` |
| `python3 benchmarks/language_write_cost/recount_and_verify.py` | PASS | `HowlFrame 165` vs `Python 266` (dirty side effect: overwrote tracked `proof_latest.json`) |
| `python3 benchmarks/v2/harness/prove_howlframe_refs.py` | PASS (tainted) | Exited 0, but built `/tmp/howlframe-v2-proof` and invoked `howlframe` (`PATH` resolved `/home/howlcipher/.local/bin/howlframe`, `HowlFrame 0.1.0`, `HFBC format: 1`) instead of repo binary |
| `python3 scripts/check_vendored_schemas.py contracts/howl/SOURCE.md ...` | PASS (4 WARN) | `5/5` schemas match pinned `howldream@dc6019a485be`; `4` warn upstream default branch has newer versions |

---

## 2. Strengths & Architectural Findings

1. **Capability-Governed Standalone Bytecode (`HFBC v2`) & Security Containment:**
   - `os.Root`-confined filesystem operations (`internal/capability/`), strict host+port `network:fetch=` with SSRF/redirect guards (`network.go`), symlink-canonical `process:exec=` with Linux `PR_SET_CHILD_SUBREAPER` process containment (`exec_contain_linux.go`), grant attenuation across `spawn`/`spawn_agent`/`route`, and AST effect-gate conformance tests.
2. **Deterministic Bytecode, Terminal Runtime & WasmGC Compiler:**
   - Canonical sorted `HFBC v2` serialization (`internal/bytecode/artifact.go`), `ValidateProgram` structural verification, `howlframe.runtime_error/v1` structured JSON errors (`internal/vm/error.go`), isolated zero-grant `terminal_app` event loop (`internal/vm/terminal_app.go`), and WasmGC backend (`internal/backend/wasmgc/`) validated with `wasm-tools`, `node`, and `wasmtime`.
   - Verified both bytecode `ShortCircuit` ([internal/bytecode/bytecode.go](file:///run/media/system/tallgeese/dev/howlframe/internal/bytecode/bytecode.go#L1031-L1050)) and the AST interpreter ([internal/vm/vm.go](file:///run/media/system/tallgeese/dev/howlframe/internal/vm/vm.go#L1272-L1292)) already short-circuit `and`/`or`.
3. **Contract Registries & Acceptance Suites:**
   - Authoritative construct/capability/bytecode registries with codegen drift tests (`cmd/codegen`) and multi-app acceptance suites (`scripts/ci_acceptance.sh`).

---

## 3. Verified Defects & Remediation Targets

1. **Repo & Build Artifact Hygiene:**
   - 2.6 MB ELF binary `codegen` committed in git (`git ls-files codegen`) alongside stray untracked `.hfbc`/`.bc.bin`/`telemetry.jsonl` artifacts in repo root and `apps/*/*.howl.bc.bin` written directly into tracked app directories by `apps/*/*_test.go`.
   - `test_unbound_test.go` overwrites tracked `test_unbound.howl` in place and runs `go run howlframe.go` per subtest.
2. **Test Speed, Assertions & Benchmark Integrity:**
   - `TestCLI_PublicInterface/check invalid` ([howlframe_cli_test.go](file:///run/media/system/tallgeese/dev/howlframe/howlframe_cli_test.go#L65-L67)) has an empty assertion body on the error message.
   - `buildHowlFrameBinaryForTest` ([howlframe_test.go](file:///run/media/system/tallgeese/dev/howlframe/howlframe_test.go#L1650-L1657)) rebuilds `howlframe` per test (`~257s` root package duration).
   - [prove_howlframe_refs.py](file:///run/media/system/tallgeese/dev/howlframe/benchmarks/v2/harness/prove_howlframe_refs.py#L32-L38) compiles `/tmp/howlframe-v2-proof` while adding `/tmp` (not binary named `howlframe`) to `PATH`, causing `run_track_a.py` to execute stale `/home/howlcipher/.local/bin/howlframe` (`0.1.0`). Benchmark proof scripts also dirty tracked `proof_latest.json` files during read-only verification runs.
3. **CLI Pipeline & Tooling Defects:**
   - [howlframe.go](file:///run/media/system/tallgeese/dev/howlframe/howlframe.go): `checkSource()` (`716-722`) and `inspectArtifact()` (`1342-1346`) miss `resumeFlagsAfterInput`; `ApplyWithContext(root, nil)` is invoked twice back-to-back in 5 call sites (`216-217`, `755-756`, `855-856`, `1250-1251`, `1295-1296`); `-compile-hfir-bc` (`219`, `269`) calls `ast.Desugar(root)` twice; `case "wasmgc"` (`983`) skips `runHFIRGate`; legacy `-run-bc` misses unified `run` VM limit flags.
   - `cmd/codegen/main.go:145` calls deprecated `strings.Title`.
   - `tools/orchestrator/orchestrator.py` (`65-79`) writes JSON bytecode into `app.json` and executes `-run-bc app.json`.
4. **Docs & Backlog Drift:**
   - Stale version/command/status strings across `docs/index.html`, `docs/installation.md`, `change_log.md`, `bugs.md #42`, `docs/design/wasm_gc_backend.md` (M6 status), and `README.md`.
5. **Re-entrancy & Performance / CI Gates:**
   - Mutable package-level globals (~25 `var`s) in `internal/backend/gogen` and `internal/backend/javascript`.
   - `improvements.md #111`: `OpAppend` ([internal/vm/vm.go](file:///run/media/system/tallgeese/dev/howlframe/internal/vm/vm.go#L3833-L3837)) copies `n` elements and charges `8*(n+1)` memory slots per append (`O(n^2)` time and memory).
   - `bugs.md #72`: `slopslint check --classify --enforce` fails (`746` vs ceiling `88`) due to both `internal/` clone duplication and experiment/benchmark Go corpora counted inside `go_production`.
   - `P1.3`: Missing structured `howlframe` verification report v0.

---

## 4. Roadmap Feasibility Summary

- **Already Shipped:** `P0.1`–`P0.6`, `P1.1`/`P1.2`/`P1.4`/`P1.6`, `P2.1`, `P2.2`, `P2.4`, `HOWLFRAME_PRODUCT_DIRECTIVE.md` Milestones 1–3, WasmGC M1–M6 (optional Firefox deferred by Owner).
- **Executing in This Initiative (Approved Option B):** Hygiene & CLI/test/benchmark/doc fixes, re-entrant `gogen` & `javascript` structs, `#111` persistent vector `append`, `#72` `slopslint` gate restoration without raising ceiling `88`, and `P1.3` Verification Report v0.
- **Deferred (Dedicated Follow-Up Work):** `#96` VM `test` runner, `#110` bounded sorting after `#111`, upstream `contracts/howl/*.schema.json` bump, `P1.5` (live LLM gated), `#90`/`#92` HFIR prod flip (`36` documented blockers), `#81`/`#83`/`#89`/`#91`/`#100`, `P2.3`, `P3.1`–`P3.4`.

---

## 5. Execution Log

1. **Checkpoint 1 (`a769f0b6`) — Hygiene, CLI Consistency, Orchestrator & Docs Reconciliation:**
   - Removed tracked `codegen` ELF binary (`git rm codegen`, ignored `/codegen`), cleaned stray root build/test artifacts, and killed the pre-existing orphan listener on `127.0.0.1:8106`.
   - Made `test_unbound_test.go`, root binary caching (`howlframe_test.go`), and `apps/*/*_test.go` hermetic inside `t.TempDir()`.
   - Restored `resumeFlagsAfterInput` on `howlframe check` and `howlframe inspect`, unified `howlframe run` / `-run-bc` execution policy limits, wired `runHFIRGate` on `--target wasmgc`, removed duplicate `ApplyWithContext` / `Desugar` passes, and replaced deprecated `strings.Title` in `cmd/codegen/main.go`.
   - Updated benchmark proof scripts (`--check` / `--write` separation, temporary local binary build on `PATH`), repaired `tools/orchestrator/orchestrator.py` + `tests/test_orchestrator.py`, and synced stale doc versions/CLI flags.

2. **Checkpoint 2 (`e2e3a2f5`) — Re-entrant Backends & Persistent Vector List (`improvements.md #111`):**
   - Eliminated mutable package-level globals in `internal/backend/gogen` and `internal/backend/javascript` using per-compilation structs verified under `-race`.
   - Replaced `O(n^2)` copy-on-append slices in `internal/vm` (`OpAppend` and list helpers) with an immutable 16-way branching persistent vector (`internal/vm/persistent_list.go`), keeping VM type-name parity and bounded memory accounting across 100k appends.

3. **Checkpoint 3 (`f9c6d7cf`) — Repository Hygiene Gate (`bugs.md #72`) & Verification Report v0 (`P1.3`):**
   - Separated non-compiler corpora (`experiments/`, `benchmarks/`, `apps/*_test.go`, `examples/*_test.go`, `experiments/*_test.go`, `benchmarks/*_test.go`) into explicit tracked scopes in `.slop/config.yml` while refactoring compiler (`go_production`: `78` <= `88`) and core test (`go_tests`: `132` <= `135`) duplication down with 0 tombstones, ratcheting `.slop/ceilings.yml` to exact active counts (`78` / `132` + explicit split ceilings `541`, `41`, `55`, `97`, `25`, `0`). Note: `slopslint ratchet main` reports `scope mismatch` strictly because `main` defines 2 scopes while `.slop/config.yml`/`.slop/ceilings.yml` now track all 8 scopes explicitly; core ceilings only decreased (`88 -> 78`, `135 -> 132`).
   - Added `internal/verification` (`howlframe.verification_report/v0`), `internal/ast.RunCatchingErrors`, structured `Checks`/`Schema`/`VerifierVersion` on `internal/hfir.VerificationEvidence`, `howlframe check --json [--profile] [--target]` (with in-memory HFBC validation on `--target=bytecode`), `howlframe inspect --report <artifact.hfbc>`, `verification_report` attachment on `agent check --json`, `interopCommands` updates, and tests in `internal/verification/report_test.go`, `internal/hfir/cas_incremental_test.go`, and `verification_report_cli_test.go`.
   - Verified before upstream merge (`f9c6d7cf`): `gofmt -l .`, `go build ./...`, `go vet ./...`, `go run ./cmd/codegen` (zero drift), `slopslint check --classify --enforce`, `slopslint tombstone check`, `scripts/check_vendored_schemas.py` (`5/5` match pinned `howldream@dc6019a485be`), all 3 benchmark `--check` scripts, `benchmarks/v2/harness` unittest (`8` passed), `scripts/test_seo.py`, `pytest -q tests/` (`12` passed), `bash scripts/ci_acceptance.sh` (all suites passed with documented `XFAIL m10_port_lost_race_stderr_shape`), `HOWL_FORBID_LOCAL_INFERENCE=1 go test -p 1 -count=1 ./...`, and `HOWL_FORBID_LOCAL_INFERENCE=1 go test -race -p 1 -count=1 ./...`.

4. **Upstream Sync (`21bfc171`) — Merge `origin/main` (`e64256e5`, PRs `#318` & `#319`):**
   - Merged `origin/main` (`#118` deterministic `terminal_app --replay`, `#119` raw-mode line editing/bracketed paste/`parseView` styles/`--color` scrolling) cleanly into `fix/audit-remediation`, keeping `sliceFromList` persistent-vector compatibility inside `terminal_app.apply` and avoiding duplicated limit wiring in `howlframe.go`.
   - Verified post-merge `gofmt`, `slopslint check --classify --enforce` (`go_production` `78`, `go_tests` `132`), `./internal/vm`, and `TestTerminalApp|TestInterop|TestVerificationReport`. Final full regression & GitHub CI (`build`, `acceptance`, `race`) execute against the immutable closeout commit before PR merge.
