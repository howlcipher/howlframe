# ledgerlens handoff

* Active milestone: Milestone 1, application 1 of 3. Requirements are frozen (2026-10-13).
* Status: implemented; 35/35 frozen acceptance tests pass locally (see `evidence/`).
* Remaining for this app: independent final review against the frozen requirements and a
  recorded accept/reject decision (D24-J), run by a reviewer who did not implement it.
* Build: `howlframe build src/ledgerlens.howl -o build/ledgerlens.hfbc`.
* Reproduce from a clean checkout: `scripts/clean_repro.sh` (uses `git archive HEAD`).
* Language changes made for this app: `read_stdin` built-in; `starts_with` empty-subject fix.
* Open observations (not fixed, no impact on acceptance): lexer double-encodes non-ASCII string
  literals; `str_len` counts runes while the spec says bytes (handled for long descriptions).
* Next: app 2, `../02-healthwatch`.
