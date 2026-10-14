# taskflow handoff

* Active milestone: Milestone 1, application 3 of 3. Requirements frozen (2026-10-13).
* Status: implemented; 32/32 frozen acceptance tests pass locally (see `evidence/`).
* Remaining for this app: independent final review against the frozen requirements and a
  recorded accept/reject decision (D24-J), by a reviewer who did not implement it.
* Build: `howlframe build src/taskflow.howl -o build/taskflow.hfbc`.
* Reproduce from a clean checkout: `scripts/clean_repro.sh` (uses `git archive HEAD`).
* Language/runtime changes made for this app: `exec` with a list argument; catch sees
  `exit_code`/`output` of a failed child (documented in `docs/reference/process_authority.md`).
* Next: independent review of all three applications, then record the Milestone 1 decision.
