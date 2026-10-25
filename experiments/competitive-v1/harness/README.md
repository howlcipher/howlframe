# competitive-v1 harness

Experiment-only infrastructure for the competitive-v1 protocol (Revision 3). It contains no HowlFrame language or
runtime code, and nothing here has made a paid model call. Python standard library only; Python 3.11 or newer.

| Component | File | What it does |
|---|---|---|
| Metering proxy | `cv1/proxy.py`, `cv1/sse.py` | Sits between a jailed CLI and its provider. Refuses server-side tools, background mode, premium tiers and unpinned models. Reserves each call's worst case before forwarding, charges actual usage from the (streamed) response, records the served model, and keeps the real API key outside the jail |
| Budget ledger | `cv1/budget.py` | Append-only, fsynced ledger. Admission reserves a run's whole ceiling; per-call reservations hold the run ceiling and the experiment cap; tool-call limit; replays exactly after a restart; stops everything if a call ever costs more than its reserved bound |
| Prices | `prices.json`, `cv1/pricing.py` | List prices checked 2026-10-23, and the worst-case bound (input tokens ≤ request bytes + allowance, at the dearest input rate, plus the request's maximum output) |
| Jails | `cv1/jail.py`, `bin/` | Bubblewrap: pinned read-only base root, one toolchain, no network, read-only tests, fresh HOME, systemd scope limits. `jail-entry` exposes the proxy socket on loopback; `task-complete` is the completion handshake; `run-acceptance-tests` runs the visible tests |
| Toolchains | `toolchains.json`, `cv1/toolchains.py` | Downloads and verifies the base root, Python 3.12.15, Go 1.25.14, Node 22.23.4 and offline docs; builds the HowlFrame compiler from a pinned commit; writes tree digests to `lock.json`. Nothing is installed system-wide |
| Agents | `cv1/agents.py` | Claude Code and Codex configuration: isolated home, vendor-default prompt, local tools only |
| Runner | `cv1/runner.py` | Admits, launches and supervises one run (45-minute limit, `task-complete`, budget stops, infrastructure faults); `run_group` launches the four arms together; `audit_surfaces` checks that a provider's arms got identical agent setups |
| Scorer | `cv1/scorer.py`, `cv1/testrunner.py`, `cv1/testkit.py`, `cv1/runtime_probe.py`, `empty/` | Fresh-jail scoring with hash-verified acceptance and held-out tests, runtime minus empty-program startup, and a delegation scan for solutions that hand work to another language |
| Records | `schema/`, `cv1/results.py` | Run and score records; every protocol §7.9 outcome has a value |
| Call archive and accounting | `cv1/proxy.py`, `cv1/transcript.py` | The proxy archives every forwarded request and response (`records/calls/`). `transcript` derives tokens by class, documentation-reading calls and bytes, active time (wall minus provider wait) and the first error the agent hit |
| Security canaries and 10× workload | `cv1/testrunner.py`, `cv1/scorer.py`, `cv1/runtime_probe.py` | During scoring: a planted credentials file (reads detected by access time in the jail's tmpfs), loopback listeners on common ports, and a search of the workspace for the canary token. The 10× workload (`workload.scale10`) is classified as success, clean error or crash |
| Analysis | `cv1/analysis.py` | §7 decision rules for H1 to H6. Task-clustered GEE intervals for proportions, task-cluster bootstrap for ratios, Bonferroni across baselines; prints the interpretations that still need a pre-freeze ruling |
| Blind grading | `cv1/blind.py` | Packets of first errors (diagnostic quality) and delegation flags with the language masked, plus a separate key |
| Jail hashes | `lock.json`, `cv1/toolchains.py --verify` | The committed tree digests of the base root, toolchains and docs. A paid feasibility run refuses a cache that does not match |
| Draw | `cv1/beacon.py` | NIST beacon pulse selection, seed and shuffle exactly as pinned in protocol §13.3 |
| Task check | `cv1/check_task.py` | Validates a task directory and scores each reference implementation through the real scorer (fresh jail, acceptance and held-out tests, workload time in 0.5–5 s); prints the seal hashes |
| Pool check | `cv1/check_pool.py` | Validates every candidate and scores its screen implementations (Python, JavaScript) in fresh jails; reports word 5-gram overlap with the HowlFrame corpora for the reviewer; prints the pool manifest digest |
| Feasibility test | `cv1/feasibility.py` | Mock mode for infrastructure checks; live mode only with a dated Owner authorization file and a complete task (held-out tests and workload). Scores every arm on acceptance, held-out and runtime |
| Mock provider | `mock/mock_upstream.py` | Deterministic Anthropic Messages and OpenAI Responses (including Codex code mode) for tests |

## Commands

```sh
cd experiments/competitive-v1/harness

# Unit, proxy, transcript and analysis tests (no jail, no network): 78 tests
python3 -m unittest tests.test_units tests.test_proxy tests.test_transcript tests.test_analysis

# Fetch and verify the pinned artifacts (about 900 MB unpacked), then build the HowlFrame arm
python3 -m cv1.toolchains --cache "$CV1_CACHE"
python3 -m cv1.toolchains --cache "$CV1_CACHE" --only howlframe --howlframe-repo /path/to/howlframe \
    --howlframe-commit <merged main SHA> --skill-file /path/to/howlplane/.agents/skills/howlframe-app-development/SKILL.md

# Integration: real CLIs in real jails against the mock (skipped without these variables): 14 tests
export CV1_CACHE=... CV1_CLAUDE_BIN=/path/to/claude/binary CV1_CODEX_DIR=/dir/with/codex/and/codex-code-mode-host
python3 -m unittest tests.test_integration

# Check a task directory: metadata, tests, and every reference scored in a fresh jail
python3 -m cv1.check_task path/to/task --out /tmp/cv1-check

# Feasibility test against the mock
python3 -m cv1.feasibility --mock tests/fixtures/toy_mock_scenario.json --task tests/fixtures/toy_task \
    --provider anthropic --out /tmp/cv1-feasibility-mock
```

`tests/fixtures/toy_task` is a harness self-test fixture, not the calibration task. The calibration task and the pilot
pool must come from the independent task author (protocol §3.2, §8).

## What a paid run still needs

The scoped different-provider review of Revision 3 (protocol §11 gate 1a) has passed
(`../FAIRNESS_REVIEW_R4_SCOPED.md`). Still needed, and none may be done by an agent alone:

1. The calibration task and its tests from the independent authors, passing `cv1.check_task`.
2. A dedicated Anthropic workspace and API key with a spend limit of at most $150 and prepaid credit with auto-reload off.
   The key goes in a `chmod 600` file read only by the proxy.
3. A dated Owner authorization file naming provider, model, task, cap and dates (`cv1/feasibility.py` docstring).

## Known limits

* **Canaries detect reads during the test suites only.** They are not active during `./build`. A read is seen only where
  the jail's filesystem records access times (it is reported as unobservable, not as clean, otherwise), and `stat()`,
  directory listings or a reset access time are not reads. Ports a task's contract names are never canaries for that
  task. A canary is reported for the blind graders, never scored as a failure.
* **The call archive is complete and unredacted.** It holds every request and response, so it contains whatever the
  agent read or printed, and takes several MB per run. Review it before publishing logs (§10). The API key is never in
  it, because the proxy adds the key after archiving the body.
* **Blind-grading masks are lexical.** Source code shown for a delegation ruling reveals its language to a reader who
  knows the syntax (`cv1/blind.py`).

* The worst-case bound assumes input tokens never exceed request bytes plus 8,192, with opaque fields (encrypted
  reasoning, signatures) counted four times. The proxy checks every response against it and stops the experiment if
  it is ever exceeded, so a wrong assumption ends spending instead of overshooting.
* The base root contains a POSIX shell, coreutils and perl-base in every arm, and the test runner's Python is mounted
  (off PATH) in every jail. The delegation scan flags solutions that use them; a blind grader decides.
* Codex 0.163.0 offers its agent-team tools with `gpt-6.1-sol` whatever the configuration says. Spawns are billed to the
  run and reported.
* The provider-side spend limit and prepaid credit are the Owner's to set up; the harness cannot verify them.
* bubblewrap shares the host kernel. It is a containment boundary for well-behaved agents, not a defence against a
  hostile kernel exploit (see HowlFrame issues #216 and #217 for the analogous HowlFrame distinction).
