> Recorded verbatim from Antigravity (`agy --sandbox --model gemini-3.8-flash-high --effort high --print`, 2026-10-25),
> reviewing `8a91af2e..a1959500`. Only local file links were made repository-relative. The author's dispositions follow.

# Scoped review R5: competitive-v1 harness additions (measurement and analysis)

**Verdict:** NEEDS REVISION

---

### Question 1: Implementation of Protocol §7.1–§7.9 and §9 in `cv1/analysis.py`

`cv1/analysis.py` faithfully implements the core mathematical definitions of protocol §7 and §9, but introduces subtle silent choices and an edge-case defect regarding infrastructure reruns.

1. **Pairing and Adjusted Fractions (§7.2):**
   * Implemented symmetrically in [`cv1/analysis.py:172-186`](harness/cv1/analysis.py). Runs are matched on `(task, provider, rep)`.
   * A side failing acceptance is assigned an adjusted held-out fraction of `0.0` (`fh = r["heldout_fraction"] if r["acceptance_passed"] else 0.0`), matching §7.2 exactly.
   * Categorization into `win` ($f_{HF} > f_B$), `shared` ($f_{HF} = f_B > 0$), `mutual_failure` ($f_{HF} = f_B = 0$), and `loss` ($f_{HF} < f_B$) strictly follows §7.2.
2. **Three H2 Forms (§7.2):**
   * Implemented in [`cv1/analysis.py:189-204`](harness/cv1/analysis.py):
     - `default` (strict win rate): $\text{wins} / \text{pairs}$.
     - `A` (shared-credit rate): $(\text{wins} + 0.5 \times \text{shared}) / \text{pairs}$; mutual failures receive 0 credit.
     - `B` (decisive rate): $\text{wins} / (\text{wins} + \text{losses})$.
   * All three forms are reported with full $W / \text{shared} / \text{mutual\_failure} / L$ counts, and `--h2-metric` defaults to `A` pursuant to the Owner decision recorded 2026-10-22 (§7.8).
3. **RMST Censoring (§7.3):**
   * Implemented in [`cv1/analysis.py:207-226`](harness/cv1/analysis.py): `WINDOW_S = 45 * 60` (2700 s). Runs where acceptance did not pass or `time_to_verified_s` is missing are censored at 2700 s. The point estimate is $\text{RMST}_{HF} / \text{RMST}_B$. Secondary concordant-pair medians and active-time medians are not calculated in `analysis.py`, but are not part of the primary decision rule.
4. **H4 Ratios and Gating (§7.4, Option C):**
   * Implemented in [`cv1/analysis.py:228-244`](harness/cv1/analysis.py) and [`cv1/analysis.py:254-262`](harness/cv1/analysis.py).
   * Option C (`--h4-gate C`) gates on Python and JavaScript only, excluding Go from the gating set (`gated = ("python", "javascript")`) and passing Go as non-gating characterization. In [`cv1/analysis.py:285-286`](harness/cv1/analysis.py), H6 evaluates consistency across the gated baselines only.
5. **H5 Maintenance (§7.5, §6):**
   * Implemented in [`cv1/analysis.py:246-250`](harness/cv1/analysis.py). Filtered to `domain == "D4"`. Scored as 1.0 if both acceptance tests and all held-out tests pass (`r["acceptance_passed"] and r["heldout_all_passed"]`), matching §6 change success.
6. **H6 Provider Consistency (§7.6):**
   * Implemented in [`cv1/analysis.py:277-290`](harness/cv1/analysis.py). Requires $\ge 2$ providers, that the pooled point estimate meets target, and that no provider's individual outcome is `"not met"`.
7. **Three-Outcome Rule and Thresholds (§7.7):**
   * Implemented in [`cv1/analysis.py:139-154`](harness/cv1/analysis.py):
     - For higher-is-better (H1, H2, H5): `met` if $\text{est} \ge \text{target}$ and $\text{lower} > \text{no\_advantage}$; `not met` if $\text{upper} < \text{target}$; else `inconclusive`. Thresholds match §7.7 (H1: target 0.80, no-advantage 0.70; H2: target 0.60, no-advantage 0.50; H5: target 0.80, no-advantage 0.70).
     - For lower-is-better (H3, H4): `met` if $\text{est} \le \text{target}$ and $\text{upper} < \text{no\_advantage}$; `not met` if $\text{lower} > \text{target}$; else `inconclusive`. Thresholds match §7.7 (H3: target 0.80, no-advantage 1.0; H4: target 2.0, no-advantage 3.0).
8. **Bonferroni Use:**
   * $\alpha = 0.05 / 3$ is applied to per-baseline hypotheses H2, H3, H4 ([`cv1/analysis.py:199, 224, 238`](harness/cv1/analysis.py)). $\alpha = 0.05$ is applied to HowlFrame-only hypotheses H1 and H5 ([`cv1/analysis.py:168, 249`](harness/cv1/analysis.py)).
9. **Interval Methods:**
   * Proportions use a task-clustered sandwich estimator (GEE with independence working correlation) on the logit scale with back-transformation via expit ([`cv1/analysis.py:94-116`](harness/cv1/analysis.py)). When variance is zero or $p \in \{0, 1\}$, a conservative Wilson interval treating each task cluster as one observation is used.
   * Ratios use a task-cluster percentile bootstrap with fixed seed and 10,000 resamples ([`cv1/analysis.py:119-136`](harness/cv1/analysis.py)).
10. **The four `OPEN_INTERPRETATIONS`:**
    * All four are genuinely open in the protocol text:
      - *H4 decision basis:* §7.4 specifies using both reference and agent-produced implementations, but omits which drives the §7.7 decision rule.
      - *H1/H5 alpha:* §7.7 mentions Bonferroni "across the three baselines", leaving the family-wise alpha for non-baseline hypotheses unstated.
      - *H6 pooled target:* §7.6 states "if the pooled estimate meets its target", leaving open whether this requires §7.7 `met` or point estimate $\ge$ target.
      - *Interval method:* §9 mentions GEE or mixed model for H2 and "the same clustering" for the rest, leaving link, working correlation, and ratio resampling procedures open.
11. **Silent Choices Made in `analysis.py`:**
    - **H4 bootstrap median pooling:** In [`cv1/analysis.py:231-236`](harness/cv1/analysis.py), `by_task` stores a list of all paired ratios across repetitions. When resampling tasks, `bootstrap` takes the median across all individual pairs in the resampled pool. A task with 6 successful pairs carries 6 times the weight of a task with 1 successful pair, rather than computing one median ratio per task first.
    - **H4 outcome on total completion failure:** If HowlFrame fails completion on all tasks, `pairs_with_runtime` is 0. In [`cv1/analysis.py:124, 142`](harness/cv1/analysis.py), `est["estimate"]` is `None`, so `decide` returns `"inconclusive"` rather than `"not met"`.
    - **H4 Bonferroni under Option C:** [`cv1/analysis.py:238`](harness/cv1/analysis.py) hardcodes `ALPHA / len(BASELINES)` ($0.05 / 3$) even when Option C is active and only 2 baselines gate.
    - **Infrastructure rerun handling:** In [`cv1/analysis.py:58-71, 174`](harness/cv1/analysis.py), `load_runs` loads all runs found on disk. If an infrastructure failure was rerun pursuant to §5 / §7.9, both the failed run and the rerun remain in `rows`. In H1 and H5, the cell is double-counted. In `pairs()`, `index = {(r["task"], r["provider"], r["rep"], r["arm"]): r for r in rows}` overwrites one run with the other based on the directory sort order from `glob.glob`.

---

### Question 2: Scoring Paths, False Verdicts, and Canaries

1. **Testrunner Counting:**
   * In [`cv1/testrunner.py:108-113, 126-133`](harness/cv1/testrunner.py), subtest failures are recorded via `addSubTest` in `result.outcomes`.
   * When subtests fail, `addSuccess` is never invoked by `unittest`.
   * `summary["all_passed"]` requires `summary["total"] > 0 and passed == summary["total"]`.
   * Any subtest failure prevents `passed` from incrementing, guaranteeing `all_passed = False`.
   * Skipped tests also prevent `passed == summary["total"]`, correctly treating skips as non-passes. No pass/fail inversion exists in `testrunner.py`.
2. **Canary Ports and Collision with Pool Contracts:**
   * **Can a canary fire on a correct solution? YES.**
     In [`cv1/scorer.py:34`](harness/cv1/scorer.py), `CANARY_PORTS = (3000, 5000, 8000, 8080, 9000)`.
     In [`cv1/testrunner.py:38-46`](harness/cv1/testrunner.py), `Canaries` binds loopback TCP listeners to these ports.
     However, several candidate tasks in domain D3 explicitly specify default loopback ports that match these canary ports:
     - `pool/d3-distributed-lock-orchestrator/prompt.md:6`: `--server <url> (optional, default: http://127.0.0.1:8080)`
     - `pool/d3-token-vault-agent/prompt.md:14, 21`: `--vault-url <url> (optional, default: http://127.0.0.1:8000)`, target: `http://127.0.0.1:9000/invoices`
     - `pool/d3-webhook-dispatcher/prompt.md:14`: `--endpoint <url> (optional, default: http://127.0.0.1:8080/events)`
     If a test or implementation exercises default connection parameters, it connects to port 8000, 8080, or 9000. The canary listener accepts the connection and marks `canary_triggered = True` in `score.json` ([`cv1/scorer.py:269`](harness/cv1/scorer.py)).
     While `scorer.py` does not fail the run outcome, **both pre-freeze validation scripts fail:**
     - [`cv1/check_pool.py:106-107`](harness/cv1/check_pool.py): `if s["canary_triggered"]: problems.append(f"screen/{arm} triggered a canary")`
     - [`cv1/check_task.py:110-111`](harness/cv1/check_task.py): `if r["canary_triggered"]: problems.append(f"{r['arm']} reference triggered a security canary")`
     Both scripts report `ok = False` and exit 1. **A valid task screen or reference implementation that adheres to the contract will be rejected as failing.**
3. **Can a Canary Miss a Read? YES.**
   * In [`cv1/testrunner.py:54-71`](harness/cv1/testrunner.py), file access is detected by comparing `st_atime` against `st_mtime - 3600`.
   * If the filesystem is mounted `noatime` (or relatime does not update), `self.atime_works` is False, so `file_read` reports `None`. In [`cv1/scorer.py:138`](harness/cv1/scorer.py), `canary_triggered` returns `None` instead of `True`, missing the read.
   * During `./build` in [`cv1/scorer.py:102`](harness/cv1/scorer.py), `_build` runs in a separate jail before `testrunner.py` is invoked. No canary file or port listeners exist during build execution; any file reads or port connections during build are missed.
   * Directory listings, `stat()` calls, or a process opening the credentials file with `O_WRONLY`/`O_RDWR` (updating `mtime`) or restoring `atime` via `utime()` will not be detected as a file read.
4. **10× Resource-Limit Classification:**
   * In [`cv1/scorer.py:162-168`](harness/cv1/scorer.py), `classify_exit` correctly maps exit code 0 to `"success"`, signal kills/timeouts/exit $\ge 128$ to `"crash"`, and $0 < \text{code} < 128$ with non-empty `stderr_tail` to `"clean_error"`.
   * If a program prints its error diagnostics to `stdout` rather than `stderr`, `timed` in [`cv1/runtime_probe.py:21`](harness/cv1/runtime_probe.py) redirects `stdout` to `DEVNULL`. `stderr_tail` is empty, causing a clean exit with a stdout message to be classified as `"crash"` instead of `"clean_error"`.
5. **Post-Run Workspace Tampering:**
   * In [`cv1/scorer.py:253-255, 283-286`](harness/cv1/scorer.py), if the workspace is modified on disk after the run ends (`current != result["final_workspace_sha256"]`), the scorer appends a note and tampering flag, but proceeds to run acceptance tests on the modified workspace. If tests pass, `outcome = "completed"`. Post-run disk modification should immediately result in `infrastructure_failure`.

---

### Question 3: Arm and Provider Leaks in `blind.py`

`cv1/blind.py` leaks both the language arm and the provider to graders in multiple places:

1. **Language Arm Leakage in Delegation Packets:**
   * In [`cv1/blind.py:65`](harness/cv1/blind.py):
     `"flags": [dict(f, file=mask(f["file"])) for f in flags]`
     `f["file"]` is masked, but `f["why"]` and `f["match"]` are unmasked.
     `f["why"]` originates from [`cv1/scorer.py:209`](harness/cv1/scorer.py) and contains:
     `"source file in another language (.py)"`, `"(ext: .go)"`, `"(ext: .js)"`, or `"(ext: .howl)"`.
     `f["match"]` contains raw interpreter names (`"python3"`, `"node"`). Graders see the exact arm extension in plain text.
   * In [`cv1/blind.py:64`](harness/cv1/blind.py), `files` includes up to 20,000 characters of the flagged source code. Python syntax (`def ...:`, `import`), Go syntax (`package main`, `func`), and JavaScript syntax are immediately recognizable to technical graders even with tool keywords masked.
2. **Language Arm Leakage in Diagnostic Packets:**
   * In [`cv1/blind.py:25-32`](harness/cv1/blind.py), `MASKS` only replaces `SyntaxError`, `TypeError`, and `ReferenceError`.
   * Standard Python exceptions (`AttributeError`, `NameError`, `ImportError`, `ValueError`, `IndentationError`, `KeyError`) and Python traceback formatting (`File "...", line ..., in ...`) are left intact.
   * Go compiler error formats (`main.<ext>:14:2: undefined: fmt.Println`) and V8 stack traces (`at Object.<anonymous> (...)`) are unmasked, revealing the language arm.
3. **Provider Leakage in Diagnostic Packets:**
   * In [`cv1/blind.py:53`](harness/cv1/blind.py), `command` contains `fe["command"]`, which is extracted from the archived request body in [`cv1/transcript.py:56, 72, 170`](harness/cv1/transcript.py).
   * For Anthropic (Claude Code), `fe["command"]` is serialized JSON from the `Bash` tool input: `{"command": "..."}`. For OpenAI (Codex), it reflects Codex tool call conventions (`exec`). Graders can distinguish providers by the tool invocation schema and wrapper error messages (e.g., Claude Code's `"Exit code: 1"` headers).

---

### Question 4: Spend Safety and Call Archive

1. **R4 Spend Guarantees:**
   * The R4 spend safety guarantees are fully preserved:
     - *Per-run ceiling ($30):* Enforced in [`cv1/proxy.py:287-294`](harness/cv1/proxy.py) via worst-case reserve before forwarding.
     - *Experiment cap:* Enforced by `Budget.reserve` in [`cv1/budget.py`](harness/cv1/budget.py) and admission gating before launch in [`cv1/runner.py:284`](harness/cv1/runner.py).
     - *Authorization:* Checked in [`cv1/feasibility.py:93`](harness/cv1/feasibility.py).
     - *Jail lock verification:* Strengthened in [`cv1/feasibility.py:94-98`](harness/cv1/feasibility.py) via `verify_lock(args.cache)`.
     - *Key isolation:* The agent never has access to the provider API keys. Keys remain outside the container jail on the proxy host.
2. **Call Archive (`cv1/proxy.py:175-188`, `open_archive`):**
   * *Secrets in bodies:* Outgoing request headers containing `Authorization: Bearer ...` and `x-api-key: ...` are attached by the proxy in [`cv1/proxy.py:307-311`](harness/cv1/proxy.py) after the incoming body `raw` has been received. The archive records only `raw` and upstream response bytes; provider API keys are never written to the archive files.
   * *Data leakage on publication:* The archive records unredacted request and response bodies containing full conversation histories and command outputs. Under protocol §10, logs are to be published. If an agent accesses local path artifacts or environment strings during a run, these will be persisted in `calls/*.req.json.gz`.
   * *Disk consumption:* Request bodies contain full cumulative message histories up to 150 tool calls. Gzipped files will consume 5–15 MB per run. For a full study (1,920 runs), archive storage will reach 15–30 GB. There is currently no disk quota or archive size limit in `open_archive`.

---

### Question 5: Hypotheses, Metrics, Targets, and Decision Rules

No hypothesis, metric, target, or decision rule has been altered:
* Table 1 hypotheses (H1 completion $\ge 80\%$, H2 wins $\ge 60\%$, H3 speed ratio $\le 0.80$, H4 runtime ratio $\le 2.0$, H5 maintenance $\ge 80\%$, H6 cross-provider reproduction) are preserved verbatim.
* §7.7 decision thresholds (Met / Not met / Inconclusive) and Bonferroni adjustments match the protocol text.
* §7.8 Owner options (H2 Option A, H4 default gating) remain unchanged.
* Protocol revisions §13.5 and §13.6 document operational harness implementations without modifying study criteria.

---

### Numbered Findings

#### 1. BLOCKING: Canary port collision with candidate contracts rejects valid tasks in `check_pool` and `check_task`
* **File:Line:** [`cv1/scorer.py:34`](harness/cv1/scorer.py), [`cv1/check_pool.py:106-107`](harness/cv1/check_pool.py), [`cv1/check_task.py:110-111`](harness/cv1/check_task.py)
* **Problem:** `CANARY_PORTS = (3000, 5000, 8000, 8080, 9000)` binds listeners on ports that are explicitly specified as default loopback targets in pool candidate contracts (`d3-distributed-lock-orchestrator` default 8080; `d3-token-vault-agent` default 8000, target 9000; `d3-webhook-dispatcher` default 8080). When a reference or screen implementation exercises the contract's default port behavior, it connects to the canary listener. `canary_triggered` becomes `True`, causing `check_pool.py` and `check_task.py` to record a problem and exit non-zero, rejecting valid candidates.
* **Fix:** Either filter `CANARY_PORTS` per task to exclude any port mentioned in `prompt.md` / `task.json`, or assign canary ports to an isolated, non-standard range unlikely to appear in candidate contracts (e.g., dedicated ports above 40000).

#### 2. IMPORTANT: Unmasked `why` and `match` fields in `blind.py` leak the language arm to graders
* **File:Line:** [`cv1/blind.py:65`](harness/cv1/blind.py)
* **Problem:** In delegation grading packets, `flags` is constructed with `dict(f, file=mask(f["file"]))`. `f["why"]` contains plain-text language identifiers from `scorer.py` (e.g. `"source file in another language (.py)"`), and `f["match"]` contains raw interpreter names (e.g. `"python3"`). These fields bypass `mask()`, directly exposing the arm to blind graders.
* **Fix:** Pass `why` and `match` through `mask()`, or sanitize `flags` to:
  ```python
  "flags": [{"file": mask(fl["file"]), "why": mask(fl["why"]), "match": mask(fl.get("match", ""))} for fl in flags]
  ```

#### 3. IMPORTANT: Infrastructure reruns are double-counted or arbitrarily selected in `analysis.py`
* **File:Line:** [`cv1/analysis.py:58-73, 174`](harness/cv1/analysis.py)
* **Problem:** Under protocol §5 and §7.9, an infrastructure failure permits one rerun of the affected run. If both the original run and its rerun exist under `RUNS_ROOT`, `load_runs` appends both to `rows`. In `h1` and `h5`, both entries are counted in the denominator. In `pairs()`, `index = {(r["task"], r["provider"], r["rep"], r["arm"]): r for r in rows}` overwrites entries based purely on `glob.glob` path sort order, non-deterministically discarding either the failed run or the rerun.
* **Fix:** In `load_runs`, inspect `infrastructure_rerun_of` (or deduplicate by cell, keeping the latest run marked `infrastructure_rerun_of` and excluding superseded runs with `outcome == "infrastructure_failure"` from analysis rows).

#### 4. IMPORTANT: Workspace modification after run completion does not fail scoring
* **File:Line:** [`cv1/scorer.py:253-255, 283-286`](harness/cv1/scorer.py)
* **Problem:** If the workspace directory is modified on disk after `runner.py` finishes (`current != result["final_workspace_sha256"]`), `scorer.py` notes the mismatch and adds a tampering entry, but still executes acceptance tests against the modified state and will award `outcome = "completed"`.
* **Fix:** If `current != result["final_workspace_sha256"]`, set `outcome = "infrastructure_failure"` or fail scoring immediately, as the state being scored is not the state the agent produced.

#### 5. MINOR: Clean errors on `stdout` are classified as crashes in 10× workload
* **File:Line:** [`cv1/runtime_probe.py:21`](harness/cv1/runtime_probe.py), [`cv1/scorer.py:166-168`](harness/cv1/scorer.py)
* **Problem:** `timed()` redirects `stdout` to `DEVNULL`. If a solution exits with code $1 \le \text{code} < 128$ and outputs its error message to `stdout` rather than `stderr`, `stderr_tail` is empty, causing `classify_exit` to classify the run as `"crash"` instead of `"clean_error"`.
* **Fix:** Capture the tail of both stdout and stderr in `runtime_probe.once()`, or check `stdout_tail` alongside `stderr_tail`.

---

### What Was Checked and Found Correct

* **Unit Test Suite:** All 73 tests in `tests.test_units`, `tests.test_proxy`, `tests.test_transcript`, and `tests.test_analysis` execute locally and pass cleanly in 15.8 s.
* **Subtest Counting Fix:** Subtest failures properly prevent `addSuccess` in Python `unittest`; `testrunner.py:133` checks `passed == summary["total"]`, ensuring that any subtest failure marks the run as failed.
* **Task-Clustered Sandwich Variance:** Hand calculations for the GEE logit Wald interval in `cv1/analysis.py:101-116` match standard sandwich formulas with small-sample $k/(k-1)$ adjustment.
* **Spend Safety Mechanics:** Proxy worst-case cost reservations, experiment budget ceilings, and API key containment outside bubblewrap jails continue to operate as verified in R4.
* **Committed Jail Hashes:** `cv1/toolchains.py:194-203` (`verify_lock`) computes tree digests against `lock.json`, and `cv1/feasibility.py:94-98` refuses paid runs if the cache deviates from the lock.

---

## Author's dispositions (Claude Opus 5.5, 2026-10-25)

Every finding was reproduced against the code and accepted. None changes a hypothesis, metric, target or decision rule.

| # | Severity | Disposition |
|---|---|---|
| 1 | BLOCKING | Fixed. `scorer.canary_ports_for` removes every port a task's `prompt.md` or `baseline_spec.md` names (`127.0.0.1:N`, `localhost:N`, `port N`) from that task's canaries. Test: `test_ports_named_by_a_contract_are_not_canaries`. The pool and calibration task were re-checked and both still pass (pool manifest unchanged, `990e2601...`) |
| 2 | IMPORTANT | Fixed. Every flag field (`file`, `why`, `match`) is masked. Masks now also cover exception names, traceback `File ... line` lines, JavaScript stack frames, exit-code headers in both CLIs' formats, and more file extensions and interpreters. `command_text` strips the tool-call wrapper (Claude Code's JSON input, Codex's code-mode script) so the provider's conventions are not shown. The remaining limit (source code reveals its language; masking is lexical) is stated in `blind.py` and the README. Tests: `test_masking_hides_the_language`, `test_command_wrappers_of_both_providers_reduce_to_the_command` |
| 3 | IMPORTANT | Fixed. `load_runs` drops a run that another run's `infrastructure_rerun_of` names, and raises on any cell that still holds two runs, so no silent choice is made. Test: `test_rerun_replaces_the_failed_run_and_unlinked_duplicates_are_refused` |
| 4 | IMPORTANT | Fixed. A workspace changed after the run is never scored; the outcome is `infrastructure_failure` and the tampering entry is kept. Test: `test_workspace_changed_after_the_run_is_an_infrastructure_failure` |
| 5 | MINOR | Fixed. The 10x probe keeps the ends of stdout and stderr, and a non-zero exit with a message on either is a clean error |
| Q1 silent choices | | H4 now takes one ratio per workload (the median over that task's pairs) and the median across tasks (test: `test_h4_weights_each_task_once`). "Inconclusive" when no runtime pair exists, and alpha 0.05/3 under option C, are now listed in `OPEN_INTERPRETATIONS` (seven in all) for an Owner ruling |
| Q2 canary misses | | Documented in the README's known limits: no canaries during `./build`; an unobservable read is reported as unobservable (`None`), not as clean; `stat()`, listings and a reset atime are not detected |
| Q4 archive | | Documented in the README's known limits: complete and unredacted; review before publication; several MB per run; the key is never in it |

Verification on the final tree: flake8 clean; **92 harness tests pass** (78 without a jail, also under Python 3.11; 14
integration with the real CLIs against the mock); `check_task` on the calibration task and `check_pool` on the pool both
pass; the jail lock verifies.


---

## Reviewer's confirmation (Antigravity, 2026-10-25, of `f214a0be`; recorded verbatim)

**Verdict:** FIXES CONFIRMED, NO NEW BLOCKING PROBLEM

---

### Finding Confirmation Table

| Item | Status | File:Line Evidence | Summary |
|---|---|---|---|
| **Finding 1** (Canary port collision) | **CONFIRMED FIXED** | [`cv1/scorer.py:35-36, 101-110, 275-281`](harness/cv1/scorer.py), [`tests/test_units.py:439-447`](harness/tests/test_units.py) | [`canary_ports_for`](harness/cv1/scorer.py) extracts ports referenced in `prompt.md` and `baseline_spec.md` matching loopback addresses or `port N`, removing them from the active canary listeners for that task. Verified by [`test_ports_named_by_a_contract_are_not_canaries`](harness/tests/test_units.py). |
| **Finding 2** (Arm & provider leaks in blind.py) | **CONFIRMED FIXED** | [`cv1/blind.py:29-42, 45-62, 82-83, 95-97`](harness/cv1/blind.py), [`tests/test_transcript.py:94-103, 125`](harness/tests/test_transcript.py) | All delegation flag fields (`file`, `why`, `match`) are masked via `mask(str(v))`. [`command_text`](harness/cv1/blind.py) strips Claude Code JSON input and Codex `exec_command` wrappers. `MASKS` regexes now cover arbitrary Python exception types, traceback frames, JS stack frames, exit codes, and additional languages. Inherent lexical masking limits are documented in [`README.md:231-233`](harness/README.md). |
| **Finding 3** (Rerun double-counting / selection) | **CONFIRMED FIXED** | [`cv1/analysis.py:84-92`](harness/cv1/analysis.py), [`tests/test_analysis.py:517-535`](harness/tests/test_analysis.py) | [`load_runs`](harness/cv1/analysis.py) identifies all superseded runs via `r["rerun_of"]`, drops them from analysis rows, and asserts that every remaining `(task, arm, provider, rep)` cell contains at most one run (raising `ValueError` on unlinked collisions). Tested by [`test_rerun_replaces_the_failed_run_and_unlinked_duplicates_are_refused`](harness/tests/test_analysis.py). |
| **Finding 4** (Workspace modification after run) | **CONFIRMED FIXED** | [`cv1/scorer.py:269-278, 292-297`](harness/cv1/scorer.py), [`tests/test_units.py:449-463`](harness/tests/test_units.py) | When `current != result["final_workspace_sha256"]`, [`score_run`](harness/cv1/scorer.py) records the tampering event, refuses to invoke `run_suite` (`score["acceptance"]` remains `None`), and immediately assigns `outcome = "infrastructure_failure"`. Verified by [`test_workspace_changed_after_the_run_is_an_infrastructure_failure`](harness/tests/test_units.py). |
| **Finding 5** (Clean errors on stdout in 10× probe) | **CONFIRMED FIXED** | [`cv1/runtime_probe.py:17, 21, 50-57`](harness/cv1/runtime_probe.py), [`cv1/scorer.py:181-184`](harness/cv1/scorer.py), [`tests/test_units.py:437-438`](harness/tests/test_units.py) | [`timed`](harness/cv1/runtime_probe.py) accepts an optional `stdout` stream; [`once`](harness/cv1/runtime_probe.py) captures tails of both `stderr` and `stdout`. [`classify_exit`](harness/cv1/scorer.py) evaluates `message = stderr_tail + stdout_tail`, correctly mapping $0 < \text{code} < 128$ with diagnostic messages on stdout to `"clean_error"`. |
| **Q1 Silent Choices** | **CONFIRMED FIXED** | [`cv1/analysis.py:44-50, 255`](harness/cv1/analysis.py), [`tests/test_analysis.py:537-543`](harness/tests/test_analysis.py) | 1. **H4 weighting:** [`h4`](harness/cv1/analysis.py) collapses repeated pairs per task to a single task median before cluster bootstrap (`by_task = {t: [statistics.median(v)] ...}`), verified by [`test_h4_weights_each_task_once`](harness/tests/test_analysis.py).<br>2. **Open choices:** Total completion failure returning `"inconclusive"` and Option C keeping $\alpha = 0.05/3$ are now explicitly listed in [`OPEN_INTERPRETATIONS`](harness/cv1/analysis.py) for Owner determination. |

---

### New Defects

**None.** 

All modified files pass syntax and linting checks (`flake8 --max-line-length=120`), and all 78 harness unit, proxy, transcript, and analysis tests execute and pass cleanly:
```
python3 -B -m unittest tests.test_units tests.test_proxy tests.test_transcript tests.test_analysis
Ran 78 tests in 15.744s
OK
```
