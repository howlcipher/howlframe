# Brief: author the competitive-v1 pilot candidate pool

An experiment compares AI coding agents (Claude Code and Codex) completing the same task in Python, Go, JavaScript
(Node.js) and HowlFrame, a new language. You are the independent task author: your provider's agents are not measured.
Four pilot tasks will later be drawn at random from your pool, one per domain, by a public randomness beacon.

Work only inside this directory, with no network. Write every task new: do not copy or adapt any task from an existing
benchmark, course, kata site or any HowlFrame repository (you have none here; do not look for one).

## Your domains

Write **7 candidates for each domain named in `DOMAINS.txt`**, varied in subject matter (do not reuse one scenario
across tasks). Domain definitions:

* **D1 CLI data processing** (greenfield): read files, validate, transform, write a report.
* **D2 HTTP JSON service** (greenfield): a small API with persistence and limits, served on loopback.
* **D3 Integration / orchestration** (greenfield): call a local HTTP dependency and/or run subprocesses with deadlines.
  The tests start any dependency themselves (e.g. a tiny `http.server` stub on a free port, or a helper script).
* **D4 Maintenance**: a change to an existing application. Write `baseline_spec.md` (the existing app's full contract)
  and `prompt.md` (the change request: a feature, a bug fix or tightened validation, written against the spec). The
  agent will receive an implementation of the baseline in its language; those implementations are written later by
  someone else, so the spec must be complete enough to implement exactly.

## Rules every candidate must meet (protocol §3.2)

1. **Language-neutral contract**: CLI arguments, stdin/stdout/stderr, exit codes, files and/or HTTP routes on
   127.0.0.1, with concrete examples. Every behaviour a test checks must be stated in the contract.
2. **Neutral feasibility**: Python 3.12, Go 1.25 and Node.js 22 can each meet it with their standard library alone,
   using only files, an HTTP server, an HTTP client, subprocesses, JSON and local file persistence. No SQL, no
   third-party packages, no compression or image codecs, no TLS, no external network. Prefer data rules every
   standard JSON parser handles the same way (e.g. treat numbers by value; avoid distinctions like `1` vs `1.0`,
   integers above 2^53, duplicate keys, or locale-dependent sorting; sort by code point).
3. **Authority label**: mark `authority_centred: true` if the task *centres* on capability, permission or access-control
   enforcement; otherwise false. Ordinary validation and an auth token check in passing do not make a task
   authority-centred. Aim for at most 2 authority-centred candidates per domain.
4. **Size**: a competent engineer needs 30 to 90 minutes; a strong agent should finish well within 45 minutes.
5. **Runtime workload**: the experiment times each solution on a fixed input "from program start to exit". So every
   contract must have a **one-shot mode that exits**: CLI tasks naturally; for services, include in the contract a
   documented batch command (for example `./run replay requests.jsonl`) that applies requests offline through the same
   logic and prints the responses. Provide `workload/generate.py` (standard library, fixed seed, writes its files next
   to itself) and `task.json` `workload.args` that run it; sizing is done later, so a medium size is fine.

## Files per candidate: `pool/<id>/`

* `task.json`:
  `{"id": "<domain>-<short-name>", "domain": "D1".."D4", "authority_centred": false, "title": "...",
    "author": "Antigravity (<model>)", "authored_utc": "...", "attestation": "<written new for this pool, not copied
    or adapted from any benchmark or HowlFrame repository task>", "summary": "<one sentence>",
    "stdlib_plan": {"python": "<modules>", "go": "<packages>", "javascript": "<modules>"},
    "workload": {"args": [...], "stdin": null, "generate": {"command": ["generate.py"], "files": {}}}}`
  (leave `files` empty; pins are added when the workload is sized).
* `prompt.md` (and `baseline_spec.md` for D4).
* `acceptance/test_*.py`: 10 to 20 standard-library `unittest` tests importing `testkit` (`kit/testkit.py`: `testkit.run`
  for CLIs, `testkit.Server` for services). The test directory is read-only; use `tempfile`. For D4, test the changed
  behaviour and enough unchanged behaviour to catch regressions.
* `screen/python/run` and `screen/javascript/run` (+ sources): implementations that pass every acceptance test. They
  prove the tests and contract are consistent and are never shown to agents. Write the JavaScript one independently of
  the Python one, from the contract.

Check each candidate with
`python3 kit/testrunner.py --tests pool/<id>/acceptance --app "$PWD/pool/<id>/screen/python/run"` (and javascript).

## Pool-level files

* `pool/SCREEN_LOG.md`: every candidate you considered, including ones you dropped, with the reason (e.g. needs a
  third-party library in some language, ambiguous, too large).
* `pool/AUTHOR_NOTES.md`: how the candidates vary, the authority labels and why, and the attestation.

Print a final report: one line per candidate (id, authority label, acceptance test count, Python and JavaScript screen
results).
