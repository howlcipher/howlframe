# Harness validation, 2026-10-24 (mock providers only)

Infrastructure checks, not results. No provider was contacted: every jail had no network except the proxy socket, and
the proxy's upstream was the local mock (`mock/mock_upstream.py`). Dollar amounts below are **simulated** from the
mock's scripted usage at list prices.

## Environment

| Item | Value |
|---|---|
| Host | Ubuntu 26.04 (toolbox), Linux 7.2.4, 12 CPUs, bubblewrap from `/usr/bin/bwrap`, no Docker or Podman |
| Harness Python | 3.14.4 (host); also compiled and unit-tested with 3.11 |
| Claude Code | 2.1.309, binary `~/.local/share/claude/versions/2.1.309` |
| Codex | 0.163.0 (`x86_64-unknown-linux-musl`), with `codex-code-mode-host` |
| Pinned artifacts | `toolchains.json`; all publisher SHA-256 values matched; tree digests in the cache's `lock.json` |
| HowlFrame arm | Compiler built from `e77e008f` with `CGO_ENABLED=0 -trimpath` (host Go 1.26.0) |

## Results on the final tree

| Suite | Command | Result |
|---|---|---|
| Lint | `flake8 cv1 tests mock` (`.flake8`, line length 120) | clean |
| Unit and proxy | `python3 -m unittest tests.test_units tests.test_proxy` | 44 passed (also under Python 3.11, and with `XDG_RUNTIME_DIR` pointing at a missing directory) |
| Integration | `python3 -m unittest tests.test_integration` (real CLIs, real jails, mock upstream) | 11 passed |
| All together | `python3 -m unittest -v tests.test_units tests.test_proxy tests.test_integration` | see "Final run" below |
| Feasibility command, mock | `python3 -m cv1.feasibility --mock tests/fixtures/toy_mock_scenario.json --task tests/fixtures/toy_task --provider {anthropic,openai}` | Both providers: 4 arms co-scheduled, identical agent surface hash across arms, Python/Go/JavaScript *completed*, the deliberately wrong HowlFrame solution *not completed (acceptance)* |

### Final run (after the review R4 fixes)

`python3 -m unittest -v tests.test_units tests.test_proxy tests.test_integration`: **55 passed in 71 s** (44 unit and proxy, 11
integration), flake8 clean, unit and proxy suites also passing under Python 3.11. The mock feasibility command, run again for
both providers, gave the same outcomes as above with identical agent surfaces across arms.

## What the integration tests established

* **Isolation.** Each arm's jail exposes only its own toolchain (the Python arm runs 3.12.15). No arm sees another
  arm's toolchain or the host's Go 1.26, Node or Python 3.14. perl-base is present in every arm (disclosed). There is no
  external network and no host path. `/tests`, the toolchain and the root are read-only.
* **Both CLIs work through the proxy from inside a jail.** Claude Code and Codex (including Codex code mode) complete a
  task, run `task-complete`, and are stopped. Every request reached the mock with the real key, injected by the proxy; the
  key never appeared in any file inside the jail's home.
* **Caps.** A run that reaches the $8 test ceiling ends as *not completed: budget* with spend at or below the ceiling.
  With a $30 cap and $30 ceiling, a second co-scheduled run is refused admission and recorded as *not started*.
* **Wall clock.** A run whose provider stalls ends as *not completed: time* at the limit.
* **Served model.** A response from a model other than the pinned one ends the run as an infrastructure failure.
* **Parity.** Within each provider, the four arms' first requests have identical system prompts, developer messages and
  tool definitions once per-session item IDs are removed. The first Codex comparison failed only on those random IDs,
  which led to the normalization.
* **Scoring.** Fresh-jail scoring with hash-verified acceptance and held-out tests, `./build` support (Go, HowlFrame),
  runtime minus empty-program startup, and outcome mapping for completed, failed, budget-exhausted and infrastructure
  runs.

## Defects found and fixed during validation

1. A call could slip past the tool-call limit if the CLI sent it before the proxy had charged the previous response. The
   proxy now waits (up to 30 s) for a run's in-flight calls to settle before checking limits.
2. Refusals were recorded after the error was sent, so the runner could learn of a budget stop after the CLI did. They
   are now recorded first.
3. Codex made one more billed call after `task-complete`. The proxy now refuses every call once the completion marker
   exists.
4. The first mock scenario reported 158K input tokens for a 15 KB request. The proxy correctly treated this as a broken
   cost bound and stopped the experiment; the scenario was corrected to realistic usage.
5. Review R4 (`../FAIRNESS_REVIEW_R4_SCOPED.md`) found six more: socket directories when `XDG_RUNTIME_DIR` does not exist,
   an asymmetric delegation scan, authorizations above the protocol limits, unpriceable usage leaking a reservation (and a
   second crash on the same input in the bound check), settlement skipped on unexpected stream errors, and the cost of a
   call in flight at a crash. All are fixed with tests.

