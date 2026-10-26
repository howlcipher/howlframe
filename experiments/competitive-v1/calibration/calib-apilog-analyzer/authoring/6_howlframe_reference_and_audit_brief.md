# Brief: HowlFrame documentation audit and calibration reference

An experiment compares AI coding agents writing the same task in Python, Go, JavaScript and HowlFrame, a new language.
In the HowlFrame arm the agent gets exactly `docs/` (README.md, SKILL.md, docs/cli.md, docs/reference/) and the
compiler `bin/howlframe`; nothing else. You are an independent author from a provider whose agents are not measured.
Work only inside this directory, with no network. Use `export PATH="$PWD/bin:$PATH"`.

## Part 1: documentation audit

SKILL.md was last edited 2026-08-20; the compiler is newer. Check that the documentation matches this compiler:

* Extract every HowlFrame program, snippet and command in `docs/` (all files). Compile or run each one that is meant to
  work. Where a snippet is a fragment, embed it in the smallest program that exercises it.
* Check every CLI flag and subcommand the docs mention against `howlframe --help` and actual behaviour.
* Check that every built-in, form or capability the docs describe exists and behaves as described.

Write `AUDIT.md`: a table of every discrepancy (file:line, what the docs say, what the compiler does, severity:
*breaks* a program that follows the docs / *misleads* / *cosmetic*). Also list examples checked and passed (counts per
file are enough), and anything you could not check. Put the test programs you used under `audit_programs/`. Do not
edit `docs/`.

## Part 2: HowlFrame reference for the calibration task

`task/prompt.md` is a task contract; `task/acceptance/` its visible tests (see `kit/`). Write a HowlFrame
implementation in `reference/` with an executable `run` (and `build`, run once from `reference/` before testing; it
must write outputs inside `reference/`). The program must be written in HowlFrame; `run` and `build` may only invoke
`howlframe`. Use only `docs/` to learn the language, as an agent would. Check it with:

```sh
python3 kit/testrunner.py --tests task/acceptance --app "$PWD/reference/run"
```

If some part of the contract cannot be expressed in HowlFrame, do not work around it in another language; stop and
document exactly what is missing. Write `REFERENCE_NOTES.md`: result (tests passed), time spent roughly, every place the
docs were wrong or insufficient while writing it (also add those to AUDIT.md), and any compiler bugs found.

Print a short final report: discrepancy counts by severity, and the reference's test result.
