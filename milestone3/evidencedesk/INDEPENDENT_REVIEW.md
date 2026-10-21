# Milestone 3 independent final review

Current decision: **ACCEPTED**, 2026-10-21; see final decision below.

Reviewer: milestone3_requirements, original independent selector/evaluator,
2026-10-20. Role-separated from app implementer and root repairs, same Codex
session. No cross-provider independence or production deployment is claimed.

Initial decision at first review: **PENDING final packaging/reproduction**, with MAX
zero-padding behavior awaiting repair. Original acceptance failures remain in
this record rather than being replaced by later green output.

## Executed independently

Built working-tree compiler with `go build -o /tmp/m1-independent-hf .`, built
native backend/projector bytecode and compiled web_app frontend via:

```
HOWLFRAME_BIN=/tmp/m1-independent-hf bash milestone3/evidencedesk/scripts/test.sh
```

All six frozen independent API/projector tests pass. Full implementation workflow
script was independently rerun and passes: live127.0.0.1 socket ownership; exact
restart preservation; Unicode/encoded task IDs and notes; physical-line errors;
real Chromium148.0.7778.96 keyboard import, filter, ordered events, claimed versus
passed, note persistence/reload, failed import state retention, inert source XSS;
actual adapted sibling Board consumes both3-record independent fixture and72-record
sanitized Plane ledger into16 tasks; startup grant denials and denied static read
preserve state; too-small finite instruction limit is enforced. No process or
remote-fetch grants were present. Transcript evidence/reviewer/workflows.txt.

Reviewer-owned additional probe `tests/reviewer_boundaries.py` finds that
MAX00003 rejects exit2 even though it denotes permitted integer3 and the input
contains exactly3 valid records. R5 specifies numeric1..1000 without a lexical
digit cap. Initial evidence is retained in evidence/reviewer/initial_boundaries.json.

## Requirement assessment

R1: backend, shared ledger module and projector are .howl/native bytecode;
frontend interaction code is web_app compiled JS with static HTML/CSS shell.
Same-origin serving and enforced loopback binding verified live. Scripts permit
explicit compiler path and scoped storage/static grants. README/user guidance
was absent at this initial review point and must be supplied before acceptance.
R2: frozen tests independently pass wrong schema/types/malformed JSON/UTF-8,
duplicate IDs and input/record ceilings; parser retains original optional fields
and unknown action values. Whole-ledger validation precedes state/store output.
R3: complete import/list/detail/note API passes including failure preservation,
reimport note retention, unknown routes/methods, encoded IDs and durable restart.
R4: genuine browser workflow and XSS checks pass against actual compiled frontend,
with no uncaught page errors; source annotations never become approvals.
R5: useful replacement of the existing importer responsibility is demonstrated by
actual local Board consumer reads, not fabricated consumer code. Original Board
source lacks modern explicit route grants and fails; see retained
board_unadapted_failure.txt. Approved R5_COMPATIBILITY_ADDENDUM permits exactly one
temporary capability declaration, no mission behavior changes and no sibling tree
edit. Source, adapter and fixture hashes are recorded in workflow output.
Not first language adoption: Board already uses HowlFrame. No upstream installation
or permanent replacement in sibling repository is claimed. MAX spelling issue
above still requires resolution.
R6: tests and independent execution pass subject to stated issue; final committed
clean-HOME/cache reproduction and user limitations/prerequisites guidance remain
pending. Host tool/environment isolation is not an OS security sandbox.

## Architecture and scope

A shared .howl ledger validator connects bounded local evidence inspection to a
real existing ecosystem consumer. It avoids a duplicate task manager and keeps
framework/domain code outside compiler/runtime. The implementation legitimately
needed narrowly tested runtime support for loopback binding, browser fetch media
headers and frontend string URL encoding; those core changes require full root
CI/regression acceptance before closure. No model/provider orchestration, command
execution, authority delegation, remote telemetry, or production/private store
access is introduced by the app.

Sources reviewed: app/backend.howl, app/frontend.howl, app/ledger.howl,
app/projector.howl, scripts/{build,run,project,test,clean_repro}.sh,
tests/verify_workflows.py, frozen requirements/acceptance and R5 addendum.

## Packaging and MAX follow-up, 2026-10-20

Root supplied README.md. Reviewer read it and confirmed build/run/project workflow,
compiler/environment options, exact capability requirements, browser dependency,
actual-adapted-consumer limitation, bounded schema/input/single-user scope,
publication I/O limitation and no-OS-sandbox claim. R1 documentation gap is closed.
Root normalized leading zeros before safe numeric MAX range validation; reviewer
reran the independent00003 probe: exit0, empty stderr, entries3/tasks2 and exactly
the two required native stores. Final evidence: evidence/reviewer/final_boundaries.json.
The original failed boundary evidence remains in initial_boundaries.json.

No outstanding application semantic defect was found in the bounded review.
Final acceptance remains pending committed clean-environment reproduction and
relevant core regression/CI completion; the six frozen tests plus independent
full browser/consumer workflow already pass on the reviewed working tree.

## Final independent acceptance decision, 2026-10-21

**ACCEPTED: D30 A (full-stack web app), B (useful ecosystem component
replacement), J (evidence-driven selection/order), against R1-R6 and the explicit
R5 compatibility addendum.** This decision supersedes the pending status above;
initial findings, failed probes and their repair history remain preserved.

Accepted implementation commit: `b028e813ac020320c938592193009692973cf079`.
The final source change catches the runner's request-body ceiling before ledger
parsing. Root discovered that payloads over300000 bytes originally escaped as
500 despite the application's262144-byte ceiling. Reviewer independently rebuilt
final artifacts and reran API/restart verification:262145,300001 and400000 bytes
all return413 payload_too_large and preserve existing imported tasks. All six
frozen tests also pass again. Exact execution transcript:
[evidence/reviewer/final_request_ceiling.txt](evidence/reviewer/final_request_ceiling.txt).

Reviewer inspected committed-archive clean reproduction output:
[../../docs/journals/evidence/fresh_milestones/m3_clean.txt](../../docs/journals/evidence/fresh_milestones/m3_clean.txt).
Root tool process confirmed actual exit0, not merely partial green output.
It builds both native artifacts and compiled frontend from the accepted commit,
runs all six frozen tests, genuine Chromium148 browser interactions, restart,
loopback listener, grant/resource denial checks, and the real adapted sibling
consumer for both3-record and72-record fixtures. It prints framework/sibling
commits plus source/adapter/fixture hashes. Fresh HOME/TMPDIR/Go caches and removed
inherited environment are documented by scripts/clean_repro.sh; installed Python
Playwright and browser paths are explicitly retained host-tool prerequisites.
This is reproducible host-tool execution, not OS security isolation.

Relevant final uncached core Go package results and four CI proof logs were
provided with root-confirmed successful tool exits under
[../../docs/journals/evidence/fresh_milestones/](../../docs/journals/evidence/fresh_milestones/).
Root also confirmed build, vet, formatting and whitespace gates. Only evidence
and closure documentation remain after this reviewed implementation commit.

Known limits are accepted as documented: local single user, bounded whole-ledger
state, subset validation, two-file projection publication can be partial on an
OS write failure, and actual Board requires the transparent temporary route-grant
adapter. No unmodified sibling compatibility, permanent sibling installation,
first language adoption, production execution or cross-provider proof is claimed.
The useful alternative projector and actual consumer contract are proven.
Stop this milestone; no speculative follow-on work is part of this acceptance.
