# Milestone 1 decision

Date: 2026-10-13. Independent final reviewer (no part in the implementation). Reviewed HEAD 2a95d364.

## OVERALL DECISION: NOT ACCEPTED (pending one blocker in ledgerlens)

Milestone 1 requires an independent reviewer to accept all three applications. Two are accepted;
ledgerlens has one reproducible violation of a written requirement.

| App | Decision | Blockers |
|---|---|---|
| 01-ledgerlens | NOT ACCEPTED | LL-1: rule-file errors are not reported in lowest-line order (REQUIREMENTS 3.2/R5) |
| 02-healthwatch | ACCEPTED | none |
| 03-taskflow | ACCEPTED | none |

Per-app detail, commands run and the adversarial checks are in `01-ledgerlens/REVIEW.md`,
`02-healthwatch/REVIEW.md` and `03-taskflow/REVIEW.md`.

## Evidence table

| Item | ledgerlens | healthwatch | taskflow |
|---|---|---|---|
| Frozen requirements untouched since 78bcf393 (`git diff 78bcf393 HEAD -- 'milestone1/*/requirements' 'milestone1/*/tests'` empty) | yes | yes | yes |
| A frozen requirements | PASS | PASS | PASS |
| B source + standalone VM artifact (runs with `env -i PATH=/nonexistent`) | PASS | PASS | PASS |
| C independent acceptance suite | 35/35 | 29/29 | 32/32 |
| D `clean_repro.sh` run by reviewer | 35/35 (13 s) | 29/29 (25 s) | 32/32 (18 s) |
| E autonomous implementation (journal vs git, plausible) | PASS | PASS | PASS |
| F README and real CLI workflow | PASS | PASS | PASS |
| G regression tests, honest limitations | PASS (notes) | PASS | PASS |
| H capability/security/limits verified by reviewer | PASS | PASS | PASS |
| I history and handoff | PASS (coarse commits) | PASS (coarse) | PASS (coarse) |
| J reviewer decision | NOT ACCEPTED | ACCEPTED | ACCEPTED |
| Reviewer adversarial probes | ~60 | ~90 | ~120 |
| Blockers | 1 (LL-1) | 0 | 0 |

## Runtime changes (86ae075c, 2a95d364)

Judged narrow and tested: `read_stdin` (new opcode, no capability, byte-exact, memory charge on
the bytecode path), `starts_with` empty-subject fix, `(exec program list)` (allow-list evaluated
on the final vector), and catch-visible `exit_code`/`output` (only through `CatchString()` in the
bytecode `try_let` handler; uncaught errors, receipts and evidence unchanged). Capability
enforcement is not weakened. `go test ./internal/vm ./internal/ast ./internal/bytecode
./internal/construct ./internal/checker`: only the 6 known exec/symlink failures in `internal/vm`.
Full `go test ./...`: 14 top-level failures in `internal/backend/gogen`,
`internal/backend/javascript`, `internal/vm` and `tools/difftest`, all exec/symlink related (the
documented host baseline); no other failure.

## Required before acceptance

1. Fix LL-1 in `milestone1/01-ledgerlens/src/ledgerlens.howl` (report the lowest-line rules error,
   including the deferred unknown-budget-category check) and correct the README limitation text.
   Add an acceptance-independent regression check for it (the frozen suite cannot be edited; use
   a separately named regression test under a non-frozen path or a revision-approved addition).
2. Re-run `clean_repro.sh 01-ledgerlens` and request a re-review of ledgerlens only.

## Carryover work (minor, none block acceptance of healthwatch or taskflow)

* Runner-level limit stops (instruction, memory, deadline) print the runtime's raw JSON error
  object on stderr, which collides with the "no raw runtime error objects, prefixed stderr"
  requirements; needs a runtime decision (prefix/structured form) or a spec clarification.
* Catchable `LIMIT_EXCEEDED` from `read_file` (memory) is reported as exit 66 `cannot read` in all
  three apps; spec 1.3 says 70.
* taskflow: validate/prove the history file before writing the provisional report, so an invalid
  or denied history does not leave or clobber a report file (TF-m1).
* healthwatch: strict integer detection (`1.0`, `1e0`); escape names in `status --format json`.
* ledgerlens: budget amounts longer than 12 digits; document limitations in README.
* Decide whether `-D` values are literal (current) or expanded (spec 3.2 wording).
* Commit granularity: implementation, journal, evidence and runtime changes landed in one commit
  per app; future milestones should commit repair iterations separately so D24-E/I can be
  reconstructed from history. Built artifacts are gitignored; evidence of the artifact is the
  clean-repro build.
* `clean_repro.sh` reuses the host Go toolchain and module cache; a container-level clean room
  would be stronger evidence for D24-D.
* `read_stdin` bytecode path charges memory after reading the whole input; the interpreter path
  does not charge. Not HFIR-lowered (documented fence).

## Stop rule

Because the milestone is not yet accepted, the autonomous loop for Milestone 1 continues only for
the single ledgerlens blocker above; once it is fixed and re-reviewed as accepted, record the
outcome and stop per directive section 6.
