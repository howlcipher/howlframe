# healthwatch handoff

* Active milestone: Milestone 1, application 2 of 3. Requirements frozen (2026-10-13).
* Status: implemented; 29/29 frozen acceptance tests pass locally (see `evidence/`).
* Remaining for this app: independent final review against the frozen requirements and a
  recorded accept/reject decision (D24-J), by a reviewer who did not implement it.
* Build: `howlframe build src/healthwatch.howl -o build/healthwatch.hfbc`.
* Reproduce from a clean checkout: `scripts/clean_repro.sh` (uses `git archive HEAD`).
* No language change was required.
* Next: app 3, `../03-taskflow` (needs `exec` failure output; see SELECTION.md risks).
