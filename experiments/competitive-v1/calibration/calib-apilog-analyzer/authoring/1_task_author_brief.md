# Brief: author the competitive-v1 calibration task

You are the independent task author for an experiment that compares how well AI coding agents complete the same
programming task in four languages: Python, Go, JavaScript (Node.js) and HowlFrame (a new language). The agents under
test are Claude Code (Anthropic) and Codex (OpenAI). You are from a different provider, which is why you write the task:
nobody whose agent is being measured may write it.

This task is the **calibration task**. It is used once, in a feasibility test that checks the test infrastructure and
measures what a typical run costs. It is **not** part of the experiment's task pool and is never reused. It should
still look like a typical pool task, because its cost is used to size the budget for the real tasks.

Work only inside this directory. Do not read any other directory on this machine, do not use the network, and do not
copy or adapt any task from an existing benchmark or from any HowlFrame repository; write a new task.

## What the task must be

* A **greenfield** program with a **language-neutral contract**: command-line arguments, stdin/stdout/stderr, exit codes,
  files, and/or HTTP routes on loopback, with concrete examples. Pick one shape:
  * D1 CLI data processing (read files, validate, transform, write a report), or
  * D2 HTTP JSON service (a small CRUD API with persistence and limits), or
  * D3 integration (call a local HTTP dependency and/or run subprocesses with deadlines).
* Solvable by **Python 3.12, Go 1.25 and Node.js 22 using only their standard libraries**, with only these effects: files,
  an HTTP server, an HTTP client, subprocesses, JSON and local persistence (files). No third-party packages, no SQL
  database, no image or compression codecs, no external network.
* **Not centred on capability or authority enforcement** (sandboxing, permission systems, access-control engines).
  Ordinary input validation and error handling are expected and fine.
* **Typical difficulty:** a competent engineer would need roughly 30 to 90 minutes; a strong agent should finish in
  well under 45 minutes. Avoid trivia, puzzles, and anything that hinges on one obscure detail. The contract must be
  unambiguous: every behaviour a test checks must be stated in `prompt.md`.

## How the agent sees it

`kit/agent_task.md` is the exact template the agent receives; your `prompt.md` replaces `{{TASK}}`. The agent writes an
executable `/workspace/run` (and optionally `/workspace/build`, run once from `/workspace` before testing). Your tests
start the program only through that `run` file.

## Deliverables

Create the directory `calibration_task/` containing:

1. `task.json`:
   ```json
   {"id": "calib-<short-name>", "domain": "D1|D2|D3", "authority_centred": false,
    "title": "...", "author": "Antigravity (<your model name>)", "authored_utc": "<ISO 8601>",
    "attestation": "<one sentence: written new for this purpose, not copied or adapted from any benchmark or HowlFrame repository task>",
    "workload": {"args": ["..."], "stdin": null}}
   ```
   `workload` is the fixed input for the runtime measurement. Files it needs go in `workload/` and are mounted
   read-only at `/workload` (so arguments refer to `/workload/<file>`); `stdin`, if not null, is a file name in
   `workload/`. Size the workload so the Python reference spends **0.5 to 5 seconds** on it, excluding process startup.
   If your task is an HTTP service, the workload instead runs the program as a one-shot command, so give the program a
   batch mode in the contract or choose a CLI-shaped task.
   If the workload files would exceed about 2 MB, include a deterministic generator script `workload/generate.py`
   (standard library, fixed seed) and commit its output too.
2. `prompt.md`: the complete task contract in Markdown, written for the agent.
3. `acceptance/test_*.py`: the visible acceptance tests. Standard-library `unittest` only, importing `testkit` (see
   `kit/testkit.py`) for `testkit.run([...])` or `testkit.Server(...)`. Each test must check observable behaviour only.
   Tests run with Python 3.12; the test directory is read-only, so write any scratch files with `tempfile`. Aim for
   10 to 20 focused tests covering the main contract and the documented error cases.
   **Do not write held-out tests**; a different author writes those from your `prompt.md`.
4. `references/python/`, `references/go/`, `references/javascript/`: a reference implementation in each language,
   each with an executable `run` (and `build` for Go, which must place its binary inside the directory), standard
   library only. Each must pass every acceptance test. These are never shown to the agents.
5. `AUTHOR_NOTES.md`: the domain and why the task is typical, the design decisions and anything deliberately left
   out, the measured workload time of each reference, and the attestation.

## Checking your work

Run the acceptance tests against each reference:

```sh
python3 kit/testrunner.py --tests calibration_task/acceptance --app "$PWD/calibration_task/references/python/run"
```

(`testrunner.py` finds `testkit.py` beside it.) For Go, run `./build` inside `references/go` first. All three must
report every test passed. Then time the workload with the Python reference.

When you finish, print a short report: the task title and domain, the number of acceptance tests, each reference's
test result, and the workload timings.
