> Recorded verbatim from Antigravity (`agy --sandbox --effort high --print`, 2026-10-24), reviewing commit `5533554f`.
> Only local file links were made repository-relative. The author's dispositions follow the review.

# Scoped fairness review R4: competitive-v1 Revision 3 and harness

**Verdict:** NEEDS REVISION

---

### Executive Summary

Revision 3 successfully formalizes the operational requirements of the `competitive-v1` protocol: it introduces bubblewrap sandbox definitions, explicit proxy metering and admission controls, NIST randomness beacon draw determinism, toolchain candidate pins, and schema validation. Crucially, Revision 3 adheres strictly to scope: it modifies **no** hypothesis (H1–H6), metric, target, decision rule, or sizing methodology.

However, the harness implementation contains one **BLOCKING** defect that causes the protocol's unit tests (`tests.test_proxy`) and the runner (`runner.run_one`) to fail with `FileNotFoundError` in environments where `XDG_RUNTIME_DIR` is set to an unmounted path, plus three **IMPORTANT** issues concerning asymmetric delegation scanning against JavaScript, missing upper-bound enforcement on Owner authorizations, and unhandled exceptions in proxy cost settlement.

---

### Answers to Scoped Review Questions

#### 1. Disposition and Implementation of G1–G12 (§13.2)
- **G1 (Feasibility test):** Closed in §8 and implemented in [`harness/cv1/feasibility.py`](./harness/cv1/feasibility.py#L1-L134). It defines a 4-run test on an unexposed calibration task outside the pool before the draw, enforcing infrastructure-only calibration. (Subject to Finding 3).
- **G2 (Dollar ceiling):** Closed in §5 and implemented in [`harness/cv1/budget.py`](./harness/cv1/budget.py#L28-L180), [`proxy.py`](./harness/cv1/proxy.py#L270-L280), [`pricing.py`](./harness/cv1/pricing.py#L86-L99), and [`prices.json`](./harness/prices.json). Replaces undefined token ceilings with a $30 ceiling per run and per-call worst-case reservations.
- **G3 (Beacon draw):** Closed in §13.3 and implemented in [`harness/cv1/beacon.py`](./harness/cv1/beacon.py#L1-L130). Pins NIST v2 endpoint, raw-byte hex concatenation, domain-ordered Fisher–Yates shuffle with SHA-256 modulo, and authority caps.
- **G4 (No container runtime):** Closed in §4 (Option b) and implemented in [`harness/cv1/jail.py`](./harness/cv1/jail.py#L1-L117) and [`toolchains.py`](./harness/cv1/toolchains.py#L1-L100). Implements bubblewrap jails over pinned base root Ubuntu 26.04.1 with per-arm toolchains and network namespaces isolated to loopback.
- **G5 (Toolchain parity and skill pinning):** Closed in §4 and implemented in [`harness/toolchains.json`](./harness/toolchains.json#L10-L65). Pins Python 3.12.15, Go 1.25.14, Node 22.23.4, and the HowlPlane skill text commit (`9681bbd4`) and SHA-256 (`725e7d7d...`).
- **G6 (Server-side tools and subagents):** Closed in §5 and implemented in [`harness/cv1/proxy.py:89-112`](./harness/cv1/proxy.py#L89-L112) and [`agents.py:16-26`](./harness/cv1/agents.py#L16-L26). Proxy refuses server-side tools, background mode, and unpinned models. Claude Code local tools are restricted (`Bash,Read,Edit,Write`); Codex features are explicitly disabled via `config.toml`, with undisableable agent-team tool calls billed and monitored.
- **G7 (Host configuration isolation):** Closed in §5 and implemented in [`harness/cv1/agents.py:50-86`](./harness/cv1/agents.py#L50-L86) and [`runner.py:333-343`](./harness/cv1/runner.py#L333-L343). Jails run with a fresh HOME per run and no host config (`--clearenv`); `audit_surfaces` enforces identical first-request surface digests across arms for each provider.
- **G8 (Served model verification):** Closed in §5 and implemented in [`harness/cv1/proxy.py:385-388`](./harness/cv1/proxy.py#L385-L388) and [`sse.py:69,115`](./harness/cv1/sse.py#L69). Served model is parsed from SSE streams and JSON payloads; mismatches trigger run termination and infrastructure failure.
- **G9 (Billing mode):** Closed in §5 and implemented in [`harness/cv1/proxy.py:393-402`](./harness/cv1/proxy.py#L393-L402). Enforces dedicated API key file (`chmod 600`) held outside the jail; dummy credentials inside the jail.
- **G10 (Runtime noise):** Closed in §6 and implemented in [`harness/cv1/scorer.py:109-138`](./harness/cv1/scorer.py#L109-L138) and [`runtime_probe.py`](./harness/cv1/runtime_probe.py#L1-L62). Scorer executes runtime workloads in a dedicated post-trial pass on a quiescent system with CPU pinning (`taskset`), recording system load averages before and after.
- **G11 (Stale R1 text in §12):** Closed in §12. Rows for R1 findings 5, 7, and 8 are clearly annotated as superseded by Revision 2.
- **G12 (Branch recency):** Closed in §11. Branch is synchronized with `main`; candidate compiler commit is pinned (`e77e008f`).

#### 2. Spend Safety
- **Cap and Ceiling Enforcement:** Every model call undergoes worst-case cost reservation ([`proxy.py:273-277`](./harness/cv1/proxy.py#L273-L277)) before forwarding. Admission requires that unspent ceilings of all active runs plus the candidate run do not exceed the experiment cap ([`budget.py:103-106`](./harness/cv1/budget.py#L103-L106)).
- **Concurrency & In-Flight Calls:** `budget.reserve` synchronizes on `_settled.wait_for(...)` for up to 30 seconds for pending calls to settle ([`budget.py:133`](./harness/cv1/budget.py#L133)), and concurrently accounts for all in-flight worst-case amounts against run ceilings and experiment caps under a thread lock.
- **Streaming & Truncation:** Cut streams or incomplete responses are charged their full reserved worst-case cost ([`proxy.py:364-365`](./harness/cv1/proxy.py#L364-L365)).
- **Model / Bound Breaches:** If reported usage ever exceeds the worst-case bound or byte bound, `budget.stop` terminates the experiment immediately ([`proxy.py:377-384`](./harness/cv1/proxy.py#L377-L384)).
- **Post-Completion Calls:** The proxy checks the `complete` marker file and refuses unpaid any call sent after `task-complete` ([`proxy.py:246-249`](./harness/cv1/proxy.py#L246-L249)).
- **Authorization & Key Isolation:** Paid live runs require an Owner authorization file validated by `check_authorization` ([`feasibility.py:41-60`](./harness/cv1/feasibility.py#L41-L60)). Real keys are read outside the jail from a `chmod 600` file ([`proxy.py:395-402`](./harness/cv1/proxy.py#L395-L402)); jails only see dummy credentials and have no external network route outside the proxy Unix socket.

#### 3. Fairness of New Operational Rules
- **Jail Layout & Limits:** Identical base image, identical memory (8G), task limit (1024), swap (0), CPU affinity, and PATH layout across all arms. Network namespaces contain loopback only.
- **Agent Configuration & Prompt Template:** Single prompt template ([`prompts/agent_task.md`](./prompts/agent_task.md)) across all arms, varying only by language name, toolchain command, and doc path. Both CLIs run with vendor defaults, isolated configs, and local tools. Tool call ceiling (150) is symmetric.
- **Runtime Measurement:** Language startup overhead is symmetrically isolated by subtracting the empty-program baseline for each arm ([`scorer.py:133-135`](./harness/cv1/scorer.py#L133-L135), [`empty/`](./harness/empty)).
- **Reservation Symmetry:** Per-call reservations are model- and size-dependent, identical across arms within a provider.
- *Discrepancy Noted:* The delegation scanner in `scorer.py` introduces an asymmetry against JavaScript (see Finding 2).

#### 4. Determinism of Draw Procedure (§13.3, `beacon.py`)
- **Determinism:** [`harness/cv1/beacon.py`](./harness/cv1/beacon.py#L48-L100) deterministically selects pulses, parses raw hex into decoded bytes, computes seeds, sorts candidate IDs by Unicode code point, and performs Fisher–Yates shuffling using big-endian integer conversions of SHA-256 digests. Two honest implementers will obtain identical output.
- **Resistance to Manipulation:** The candidate pool manifest SHA-256 is committed to git ≥ 24 hours prior to the beacon pulse timestamp. The pulse is signed by NIST's HSM from hardware entropy and cannot be predicted or mined in advance.

#### 5. Stability of Hypotheses, Metrics, Targets, and Decision Rules
- Revision 3 changes **none** of H1–H6, primary measures, Bonferroni-adjusted $\alpha/3$ thresholds, RMST ratio boundaries, the 45–80 task sizing range, or the three-outcome decision rules.

---

### Numbered Findings

#### Finding 1 (BLOCKING): Unchecked `XDG_RUNTIME_DIR` breaks test suite and runner execution
- **File:Line:** [`experiments/competitive-v1/harness/tests/test_proxy.py:39`](./harness/tests/test_proxy.py#L39) and [`experiments/competitive-v1/harness/cv1/runner.py:216`](./harness/cv1/runner.py#L216)
- **Problem:** Both modules attempt to construct temporary directories with `dir=os.environ.get("XDG_RUNTIME_DIR") or "/tmp"`. In many standard Linux server, sandbox, container, or subshell environments, `XDG_RUNTIME_DIR` is set in the environment (e.g. `/run/user/1000`) but the directory does not actually exist on the filesystem. Because the string is non-empty, `tempfile.mkdtemp` attempts to create the path under `/run/user/1000`, immediately raising:
  `FileNotFoundError: [Errno 2] No such file or directory: '/run/user/1000/cv1t-...'`
  Executing the repository validation command `python3 -m unittest tests.test_units tests.test_proxy` immediately fails with 16 errors. Furthermore, [`runner.py:216`](./harness/cv1/runner.py#L216) fails at runtime whenever `socket_root` is not explicitly passed.
- **Fix:** Verify that the directory actually exists on disk before selecting it:
  ```python
  xdg = os.environ.get("XDG_RUNTIME_DIR")
  tmp_dir = xdg if (xdg and os.path.isdir(xdg)) else "/tmp"
  ```

#### Finding 2 (IMPORTANT): Asymmetric delegation scanner flags standard JavaScript imports
- **File:Line:** [`experiments/competitive-v1/harness/cv1/scorer.py:154-160`](./harness/cv1/scorer.py#L154-L160)
- **Problem:** Line 155 explicitly exempts Python source files from flagging the word `python`:
  `if arm == "python" and why == "names another interpreter" and ext == ".py": continue`
  However, no such exemption exists for the JavaScript arm. Any idiomatic Node.js solution using modern standard library imports (such as `import fs from "node:fs"` or `require("node:path")`) matches `\bnode\b` in `DELEGATION_PATTERNS`. Because `main.js` is not named `run` or `build`, line 158 appends a spurious delegation flag for every valid JavaScript run using `node:` prefixes. Go and HowlFrame are unaffected because `go` and `howlframe` are not in `DELEGATION_PATTERNS`.
- **Fix:** Generalize the self-referential interpreter exemption in [`scorer.py:154-160`](./harness/cv1/scorer.py#L154-L160):
  ```python
  if why == "names another interpreter" and ext in own and m.group(0) in _arm_interpreters(arm):
      continue
  ```

#### Finding 3 (IMPORTANT): `check_authorization` does not enforce protocol §8 feasibility hard caps
- **File:Line:** [`experiments/competitive-v1/harness/cv1/feasibility.py:58-59`](./harness/cv1/feasibility.py#L58-L59)
- **Problem:** Protocol §8 specifies hard limits of $150 total and $30 per run for the feasibility test ("Hard limits: $150 for the whole test and $30 per run"). However, `check_authorization` only validates `0 < float(auth["run_ceiling_usd"]) <= float(auth["cap_usd"])`. If an authorization file is provided with `cap_usd: 500` and `run_ceiling_usd: 100`, `feasibility.py` accepts it and initializes `Budget` and Claude's `--max-budget-usd` above the preregistered limits.
- **Fix:** Add explicit upper-bound checks against protocol §8 limits in `check_authorization`:
  ```python
  if float(auth["cap_usd"]) > 150.0 or float(auth["run_ceiling_usd"]) > 30.0:
      raise NotAuthorized("authorization limits exceed protocol §8 feasibility caps ($150 total, $30/run)")
  ```

#### Finding 4 (IMPORTANT): Malformed usage blocks bypass `budget.settle` and leak reservations
- **File:Line:** [`experiments/competitive-v1/harness/cv1/proxy.py:355-357`](./harness/cv1/proxy.py#L355-L357)
- **Problem:** In `_Handler._settle()`, `pricing.call_cost(p.prices, p.pinned_model, usage)` is called without a `try/except` guard. If upstream returns an unexpected type (e.g. `cache_creation` as a string or list), `_anthropic_cost` or `_openai_cost` raises an unhandled exception before `p.budget.settle(token, ...)` is called. The reservation token is leaked in `_reservations` and `worst` remains indefinitely in `run_pending`.
- **Fix:** Wrap `pricing.call_cost` in a `try...except Exception as e:` block and fall back to `cost = worst` with `basis = f"pricing error ({e}); charged reserved worst case"`, ensuring `budget.settle` is always invoked.

#### Finding 5 (MINOR): `self._settle` outside `try/finally` in proxy forwarding
- **File:Line:** [`experiments/competitive-v1/harness/cv1/proxy.py:303-348`](./harness/cv1/proxy.py#L303-L348)
- **Problem:** `_Handler._forward()` catches only `(OSError, http.client.HTTPException)`. Line 348 (`self._settle(...)`) is outside the `try/finally` block. Any unexpected exception during SSE parsing or streaming bypasses `_settle()`, abandoning the reserved token.
- **Fix:** Place `self._settle(...)` in a `finally` block or catch `Exception` during the stream-pumping phase.

#### Finding 6 (MINOR): Mid-call crash releases in-flight reservation upon restart
- **File:Line:** [`experiments/competitive-v1/harness/cv1/budget.py:79-81`](./harness/cv1/budget.py#L79-L81)
- **Problem:** `budget.reserve()` maintains in-flight reservations in memory without writing them to `ledger.jsonl`. If power is lost or the host process is killed via `SIGKILL` while a provider call is in flight, `_replay()` releases all active runs upon restart. Because the in-flight call's actual cost was never settled to disk, that call's spend is omitted from `self.spent`, while the run's unspent ceiling is returned to the budget. While re-running the same run ID is rejected by `duplicate_run`, other unadmitted runs could consume the released headroom.
- **Fix:** Document this failure mode in `budget.py` recovery notes, recognizing that the provider-side prepaid account cap acts as the final safety stop.

---

### What Was Checked and Found Correct

1. **Hypothesis and Sizing Invariance:** Confirmed that Revision 3 made no modifications to H1–H6, targets (80%, 60%, 0.8, 2×, 80%), power calculations ($0.80$ joint power at $\alpha=0.05/3$), or the 45–80 independent task planning range.
2. **API Key Isolation:** Verified that bwrap runs with `--clearenv` and binds dummy credentials inside the jail (`ANTHROPIC_API_KEY` and `CV1_OPENAI_KEY`). Proxy replaces credentials outside the container; external network egress is blocked by loopback isolation.
3. **Worst-Case Cost Calculus:** Verified that `pricing.worst_call_cost` prices every input token up to the byte/context bound at the dearest input/cache-write rate ($8/MTok for Claude; $5/MTok long-context for Codex) and assumes maximum allowed output tokens ($20/MTok for Claude; $15/MTok for Codex). If reported tokens exceed bounds, the proxy shuts down the run immediately.
4. **Draw Procedure Determinism:** Verified that [`beacon.py`](./harness/cv1/beacon.py) implements NIST Randomness Beacon v2 pulse extraction, raw byte hex decoding, big-endian SHA-256 Fisher–Yates shuffling, and authority capping with zero floating-point or library RNG dependencies.
5. **Schema Coverage of Protocol Outcomes:** Verified that [`run_result.schema.json`](./harness/schema/run_result.schema.json) and [`score_result.schema.json`](./harness/schema/score_result.schema.json) include all §7.9 outcome states (`completed`, `not_completed_acceptance`, `not_completed_time`, `not_completed_budget`, `not_completed_tool_calls`, `infrastructure_failure`, `not_autonomous`, `expressibility_failure`, `not_started`).
6. **Isolated Scoring & Empty Program Startup:** Verified that scoring executes against hash-verified frozen test fixtures in a fresh jail without agent or network access. Confirmed that empty programs exist for all four arms ([`harness/empty/`](./harness/empty)) and that startup latency is subtracted from runtime measurements.
7. **Passing Unit Tests (under valid TMPDIR):** Verified that with `XDG_RUNTIME_DIR=""`, all 24 unit tests in `tests.test_units` and all 16 proxy tests in `tests.test_proxy` pass cleanly.

---

## Author's dispositions (Claude Opus 5.5, 2026-10-24)

All six findings were reproduced against the code and accepted. None changes the protocol's methodology.

| # | Severity | Disposition | Where |
|---|---|---|---|
| 1 | BLOCKING | Accepted. Socket directories use `XDG_RUNTIME_DIR` only if it exists, else `/tmp` (`runner.short_socket_root`, also used by the proxy tests). The unit and proxy suites now pass with `XDG_RUNTIME_DIR=/nonexistent/dir` | `harness/cv1/runner.py`, `harness/tests/test_proxy.py` |
| 2 | IMPORTANT | Accepted. The delegation scan now ignores an arm's own interpreter name in every file of that arm (source, `./run`, `./build`), so `node:` imports and Python shebangs are treated alike; tests cover both arms and a true positive | `harness/cv1/scorer.py`, `tests/test_units.py` `Delegation` |
| 3 | IMPORTANT | Accepted. An authorization above $150 total or $30 per run is refused; raising either needs a protocol revision | `harness/cv1/feasibility.py`, `tests/test_units.py` `Authorization` |
| 4 | IMPORTANT | Accepted. An unpriceable usage block is charged the reserved worst case and still settles. Fixing it exposed a second crash in the post-charge bound check on the same input, now skipped unless usage priced cleanly | `harness/cv1/proxy.py`, `tests/test_proxy.py` |
| 5 | MINOR | Accepted. Any exception while forwarding or streaming is caught, and settlement runs in `finally` | `harness/cv1/proxy.py` |
| 6 | MINOR | Accepted, and fixed rather than documented: each reservation is written to the ledger before the call is forwarded; on replay, a reservation with no settled call is charged at its worst case, once | `harness/cv1/budget.py`, `tests/test_units.py` |

---

## Confirmation (Antigravity, 2026-10-24, reviewing commit `8c0babec`)

> Recorded verbatim; local file links made repository-relative.

### R4 confirmation: harness fixes in 8c0babec

Verdict: CONFIRMED

- Finding 1: RESOLVED — [`experiments/competitive-v1/harness/cv1/runner.py:41-44,222`](./harness/cv1/runner.py#L41-L44) and [`experiments/competitive-v1/harness/tests/test_proxy.py:19,40`](./harness/tests/test_proxy.py#L19): [`short_socket_root()`](./harness/cv1/runner.py#L41-L44) validates `os.path.isdir(xdg)` before using `XDG_RUNTIME_DIR` and falls back to `/tmp`, allowing the test suites and runner to execute without `FileNotFoundError` when `XDG_RUNTIME_DIR` is set to a nonexistent path.
- Finding 2: RESOLVED — [`experiments/competitive-v1/harness/cv1/scorer.py:157-158`](./harness/cv1/scorer.py#L157-L158): The interpreter self-reference exemption was generalized to filter matches against `_arm_interpreters(arm)` across all scanned files, eliminating false-positive delegation flags for JavaScript `node:` standard library imports while continuing to flag cross-language delegation.
- Finding 3: RESOLVED — [`experiments/competitive-v1/harness/cv1/feasibility.py:34-35,63-65`](./harness/cv1/feasibility.py#L34-L35): Explicit constants `MAX_CAP_USD = 150.0` and `MAX_RUN_CEILING_USD = 30.0` are strictly validated in `check_authorization`, raising `NotAuthorized` if an Owner authorization specifies limits exceeding protocol §8 hard caps.
- Finding 4: RESOLVED — [`experiments/competitive-v1/harness/cv1/proxy.py:357-362,384`](./harness/cv1/proxy.py#L357-L362): `pricing.call_cost` is enclosed in a `try...except Exception` handler that charges the reserved worst-case cost on unpriceable usage, settles the reservation without leaking, and guards against secondary crashes in token bound validation.
- Finding 5: RESOLVED — [`experiments/competitive-v1/harness/cv1/proxy.py:337-348`](./harness/cv1/proxy.py#L337-L348): Upstream connection, forwarding, streaming, and SSE parsing are caught under `except Exception`, and `self._settle` is relocated into the `finally` block to guarantee settlement under any stream-pumping or socket failure.
- Finding 6: RESOLVED — [`experiments/competitive-v1/harness/cv1/budget.py:70-73,84-89,160-164`](./harness/cv1/budget.py#L70-L73): In-flight reservations are persisted to disk via `kind: reserve` entries with UUID tokens before request forwarding; on crash recovery, `_replay()` charges open reservations at their worst-case cost, settles them, and appends a `kind: call` record to avoid double-charging on subsequent re-runs.

New defects in changed lines: None
