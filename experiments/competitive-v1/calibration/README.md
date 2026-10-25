# Calibration task for the feasibility test

`calib-apilog-analyzer/` is the calibration task of protocol §8: used once, by the feasibility test, as an
infrastructure check and budget calibration. It is outside the pilot pool and is never reused. It is a D1-shaped CLI
task (NDJSON access-log validation and aggregation into a JSON report), not authority-centred.

No paid model call was made to produce it: both authors ran on the Owner's existing subscription logins (Antigravity's
Google account and Codex's ChatGPT login), with no API key.

## Who wrote what

| Part | Author | Saw |
|---|---|---|
| Contract (`prompt.md`), acceptance tests, references in Python, Go and JavaScript, workload generator, `AUTHOR_NOTES.md` | Antigravity CLI 1.4.7, `gemini-3.8-flash-high`, `--sandbox`, 2026-10-24 | The briefs in `authoring/`, the agent prompt template and the test kit |
| Held-out tests (`heldout/`), `HELDOUT_NOTES.md` | Codex CLI 0.163.0, `gpt-6-luna` (ChatGPT login), `workspace-write` sandbox, 2026-10-24 | The brief, `prompt.md`, the acceptance tests and the test kit; never the references |
| Briefs, revision requests, checks | Claude Opus 5.5 (operator) | Everything |

The operator wrote no task content. Its only file changes were removing build outputs (`references/go/app`,
`__pycache__/`) and the generated workload file. Every request the operator sent is in `authoring/`, numbered in order,
with each author's report beside it.

Neither author is the feasibility test's trial agent (Claude Code, `claude-opus-5-5`). OpenAI is a trial provider in the
pilot but not in the feasibility test, so Codex writing this task's held-out tests satisfies §3.3 ("a different author
and provider from the acceptance tests") here; the pilot pool needs its own arrangement for held-out authorship.

## Revision history

1. **Task revision 1** (operator check): the Go build script hardcoded a path in the authoring directory, and the
   JavaScript reference ran the 100,000-record workload in 0.28 s, below §3.3's 0.5–5 s. The author removed the path and
   resized the workload to 250,000 records. The workload is now generated (`workload/generate.py`, seed 42) and pinned by
   SHA-256 in `task.json`. The harness regenerates it with the pinned CPython 3.12.15 and refuses a mismatch. The same
   pin was reproduced with CPython 3.14.4.
2. **Task revision 2** (held-out tests): Python passed all held-out tests, while Go and JavaScript each failed the same
   two. Both failures were contract ambiguities that favoured Python:
   * Whether `1.0` is an integer. Node's `JSON.parse` cannot tell `1.0` from `1`.
   * Whether a directory given as `--input` counts as unreadable.

   The author chose a value-based integer rule: a finite whole number up to 2^53 − 1, so `1.0` is valid. It also made
   a directory input exit 2. It stated both rules in `prompt.md`, aligned the references, and added two acceptance
   tests.
3. **Held-out revision 1:** Codex updated its tests to the revised contract and added coverage of the new rules. Count:
   24 → 29.

## Check (`python3 -m cv1.check_task`, pinned toolchains, scoring jails, 2026-10-24)

| Reference | Outcome | Acceptance | Held-out | Workload, startup subtracted | Delegation flags |
|---|---|---|---|---|---|
| Python | completed | 18/18 | 29/29 | 2.01 s | none |
| Go | completed | 18/18 | 29/29 | 2.43 s | none |
| JavaScript | completed | 18/18 | 29/29 | 0.59 s | none |
| HowlFrame | no reference | | | | |

Seal (SHA-256 over each tree manifest, as printed by `check_task`):

| Tree | SHA-256 |
|---|---|
| `acceptance/` | `3106ad6a747b4cacaef5bf7846e5b58a8fbb2caaf9bc918f839a69c6514918ac` |
| `heldout/` | `daa022eef661af4a8d45bd052b1e743cabd465725c32186dc1c0b89267301d04` |
| `workload/` (generator plus generated file) | `75fc8a3dad91d20f0b1dd89864bf2823a001ff1d1eaf794d6ec2a296d1b30302` |

## Limits

* **No HowlFrame reference.** §3.2 step 5 asks for references in each language for drawn tasks. For this task, the
  HowlFrame run is scored without one, and a HowlFrame failure would not show whether the task is expressible.
* **The held-out tests are committed in this repository.** Trial agents cannot read them, because the jails have no
  network and no host paths. The seal is the published hash above.
* **The JavaScript workload time (0.59 s) is near the lower bound.** A faster machine could bring it below 0.5 s. Runtime is a secondary measure, and the feasibility test reports it only as an infrastructure check.
