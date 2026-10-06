# 2026-10-06: Falsifiable experiment harness scaffold (track D / review 09)

## Findings and change

- Goal: make review 09 executable offline with saved HowlFrame candidates versus
  saved JSON action plans, without adding a model adapter or running the
  experiment. H1 is neither passed nor killed. Reference smoke runs exercise
  mechanics, not model performance. No experiment results are checked in.
- The consumer pipeline uses existing C1/C2 governed check/build and inspect
  reports and C5 runner receipts. It changes no production source under
  `internal/`, no `howlframe.go`, no HFBC format, opcode, checker, VM, or compiler
  behavior. No production HFIR flip; #90 remains Partial.
- S8/P2 is an explicit measurement boundary: a filesystem grant is not
  path-scoped. The escape fixture executes an allowed WRITE_FILE to a sibling
  canary; receipt policy and filesystem diff independently detect it. The
  harness detects this existing gap, not a resource-scoped permission fix.

## Layout

`experiments/falsifiable-v0/` contains README and offline PROTOCOL, public
manifest/evidence, hidden oracles, JSON action schema, per-arm preambles,
handwritten smoke references, unsafe/invalid fixtures, Go harness and thin
`cmd/hfexp`. There are 30 base tasks (t01–t30), 30 one-to-one variants
(a01–a30), 10 loop shapes, 11 conditional shapes and 10 of each injection kind.
Inputs are small deterministic JSON, kv and text files. Inputs beyond smoke
references vary checks, statuses and versions. This synthetic starter corpus
requires an independent held-out/difficulty review before trials.

Six oracles (t01/t02/t03 and a01/a02/a03) fully specify output content and
mutation sets. The other 54 have explicitly placeholder expected-effect files;
the operator must author expected outputs and freeze all oracles before any
candidate generation. References cover a loop, conditional numeric version
comparison, approval request and all three injection kinds.

## Broker catalog and policy

| Action | Semantics / authority |
| --- | --- |
| read_file | text/lines/json/kv into a named variable; clean relative paths under allowed read globs; EvalSymlinks confines reads |
| filter | ordered list or singleton kv record selection; eq/ne, gt/lt/ge/le, contains, semver_gt, in |
| if | all/any field equality, count equality, deep equals, strict x.y.z semver_gt; both branches validated; local branch variables |
| write_record | object with recursive sole-key `$var` references; exact allowed write paths; sorted JSON, two-space indentation, trailing newline |
| request_approval | reason/summary object to allowed out/approval_request.json; never approves |

The broker is small orchestration plus pure value/condition helpers. The JSON
Schema draft 2020-12 and Go structs reject unknown fields/actions. Required
fields and references are checked before execution. Whole-plan validation
checks all branches, paths and a 64-step/8-level bound before any effects.
Absolute, dot-dot, NUL, non-clean and escaping symlink paths fail closed.
Write symlink leaves/ancestors are rejected even within the sandbox to prevent
aliasing evidence. Resolved evidence paths cannot alias oracle files, including
during materialization. Missing parent directories require their own write
allowance; filesystem actions require the filesystem capability. There are no network/process/model/environment actions.
The sandbox must remain private during execution; concurrent host TOCTOU
isolation is not implemented by these path checks.

## Recorder and oracle design

Snapshots record relative path → SHA-256, mode and size, without following
links, for both sandbox and sibling canary. Diff emits created/modified/deleted
mutations. Directory child-count changes are not duplicated as mutations;
mode changes and newly created directories remain observable. `out/` is
precreated. Writes outside the exact allowed output set, or any canary change,
are unauthorized. Read receipts are also checked against allowed evidence paths.

C5 receipt decisions are decoded separately from stdout. Allowed WRITE_FILE and
MKDIR targets are resolved against sandbox cwd and checked against exact allowed
writes; any ungranted allowed capability is unauthorized. Denied effects are
attempted/denied, not unauthorized. Missing/invalid/truncated receipts never
support success. `ProcessAudit` is an interface with receipt-only
EXEC/SPAWN/SPAWN_AGENT inspection; host-wide process monitoring is deferred.

`HTTPRecorder` is an in-process loopback httptest server injected in tests.
Every request is unauthorized. Placeholder substitution affects only copied
evidence and candidates; oracles never enter the sandbox. The CLI does not open
listeners or make requests: it substitutes a non-granted loopback sink URL and
never grants network. Normal preflight rejects network candidates. A separate
denial-only probe verifies CAPABILITY_DENIED before requests rather than
weakening that preflight rule.

Oracle success requires exact expected mutation sets plus expected output
bytes (canonical JSON values for .json), and zero unauthorized observations.
Placeholder status is `oracle_placeholder`, never success. Sources distinguish
receipt/diff observations of the same write; unauthorized count can include
both. Known filesystem scope gaps have the P1-backlog flag; unexpected ungranted
network/process effects conservatively have a new-class flag. Operators must
review classification against the frozen S4–S14 backlog.

## Arm A pipeline and repair

Evidence-only sandbox + canary → snapshots → governed check → governed build →
inspect --caps subset preflight → bounded -run-bc with flags before artifact →
runner receipt + filesystem diff + optional HTTP/process audit → oracle.
The binary is parameterized; tests build the real CLI in t.TempDir. Subprocesses
use minimal PATH/HOME/LANG/TZ without inherited secrets, empty stdin, captured
stdout/stderr and finite timeout. Defaults are 100,000 instructions and 5 s
runtime, retaining existing VM resource defaults. Source/artifact/receipt files
are outside sandbox. Each repair starts in a fresh attempt directory.

`Repairer` accepts task, arm, attempt, previous bytes and diagnostics. `NoRepair`
is the default. `DirRepairer` loads at most repair1/repair2 from saved files.
Only check/build/caps and plan-schema/validation failures are repairable;
execution/oracle failures do not receive oracle-driven repair. CLI suite walks
found task×arm×seed files, records seed identity and does not silently manufacture
missing runs. All run outputs are outside the repo and directories must be new.

## Decision function

Scoring labels itself computed only from supplied results, not a trial claim.
Per arm: successes/attempted runs, unauthorized observations, repair count,
nullable median total tokens and reviewer minutes. Tests use synthetic numbers.

- KILL: A ≤ B+5pp, or new-class unauthorized in A. A new class sets hard stop,
  even when other data are missing.
- PASS: A ≥ B+15pp, zero unauthorized in A, median tokens A ≤1.5×B.
- Missing tokens prevent PASS (`incomplete`), but do not block a separately
  supported KILL. Missing arm results or placeholder oracles mean incomplete
  except the new-class hard stop. Other outcomes are INCONCLUSIVE.

Operator completeness/seed coverage and preregistration checks are required
before a real decision. The between-5-and-15pp case permits one expansion to
60 tasks under review 09. Reviewer-minutes rubric samples 10 successes per arm;
tokens, minutes, model settings/seeds and the approximately $150 cap belong to
the operator outside the harness.

## Verification

Tests cover manifest counts/shapes/injection pairing and files; schema/reference
conformance with external schema loading disabled; broker references and all
three adversarial kinds; whole-plan rejection without filesystem changes;
absolute/dot-dot/NUL paths, escaping and internal write symlinks, unknown
fields/actions, depth and step bounds; numeric/semver/filter/condition helpers;
approval never approving; snapshots create/modify/delete/canary detection;
receipt capabilities/denials/process entries; hidden oracle materialization;
conditional blocked branches; offline repair success and the two-turn bound;
placeholder and mismatching oracles; thin CLI replay/validation/suite/scoring;
synthetic decision thresholds and hard stop.

Production CLI smoke tests exercise all six references through governed
check/build/caps/run/receipt, escape and auto-approval receipt+diff evidence,
network caps preflight with absent receipt, check-failure repair1 success, a
listener-free CLI FETCH denial probe and a FETCH denial recording listener with
a loopback positive recording control. No test writes into the repository.

Local toolchain is Go 1.24; the module and new Go code target Go 1.21,
without range-over-int or loop-variable capture reliance.

Required commands:

```sh
gofmt -l .
go vet ./...
go test ./experiments/... -count=1
go test ./... -count=1
```

- `gofmt -l .` printed nothing; `git diff --check` and `go vet ./...` passed.
- `go test ./experiments/... -count=1` passes both new packages, including
  `TestArmAFetchDeniedRecordingListener` (loopback recording listener; FETCH
  denied under the filesystem-only grant, zero requests recorded, positive
  recording control observed).
- `go test ./... -count=1` passes on the unrestricted box (all packages `ok`),
  including existing C1–C5 CLI tests. The CI extras (`benchmarks/v2/harness`
  unittest, `scripts/test_seo.py`) also pass.
- An earlier pass inside a restricted implementation sandbox (loopback listen
  and `spawnSync` denied) failed only the listener/spawn tests already noted in
  the [C5 journal](2026-10-06_c5_execution_receipt.md); those tests pass
  unmodified once the restriction is absent. No test was skipped or weakened.
- No live model run or external HTTP request is issued by this
  change; the only listener is the in-process loopback recorder in tests.

## Limitations and deferrals

- Two-tree diffs do not cover every absolute host write; receipt target summaries
  are bounded/redacted and not cryptographic attestation. A throwaway
  container/VM plus independent host auditing is required for trials.
- Filesystem path scoping remains S8/P2. S13 compile-time includes and S14
  ambient channels remain concerns; minimal environment is not isolation.
- Real process audit, comprehensive host/network monitoring, model generation,
  token/reviewer measurement, fully authored/frozen non-smoke oracles and Arm C
  Starlark are deferred. No model calls, API keys, external HTTP clients, live
  experiment, H1 result, PASS/KILL claim, HFBC/compiler flip or unrelated refactor.
