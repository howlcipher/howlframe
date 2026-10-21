# EvidenceDesk frozen requirements

Independent selector/evaluator: agent milestone3_requirements, 2026-10-20.
This specification predates implementation. No silent weakening is permitted.

## R1 Packaging and execution

Provide app/backend.howl, app/frontend.howl rooted in web_app, a HowlFrame
projector CLI source, scripts/build.sh, scripts/run.sh and scripts/project.sh,
README.md, and built standalone .hfbc artifacts. Browser JS is compiled from
HowlFrame; HTML/CSS shells may be handwritten. No handwritten JS domain/interaction
logic or generated non-HowlFrame server. Scripts accept HOWLFRAME_BIN; build may
build a local compiler when unset. run accepts EVIDENCEDESK_PORT (default 8093),
EVIDENCEDESK_DATA (persistent state path), and EVIDENCEDESK_STATIC (built assets).
Serve same-origin HTML at / and compiled JavaScript at /app.js. Bind loopback
only; add narrow runtime host binding if needed. No remote fetches, model calls,
process spawning or sibling/private production reads. Document local-use scope,
necessary grants, denied grants and instruction limits; do not claim OS isolation.

## R2 Ledger input contract

A ledger is UTF-8 JSONL, maximum 262144 raw bytes and 1000 nonblank records.
Empty/whitespace-only input is invalid. Blank lines are ignored. Each nonblank
line is a JSON object with schema exactly ai.evidence_entry/v1 and nonempty string
entry_id, task_id, agent_id, action. IDs have at most 120 characters. All other
fields are optional and retained in task events. Unknown action values are valid,
retained literally, and never imply successful verification. Duplicate entry_id
within one ledger is invalid. Missing/wrong typed required fields, malformed JSON,
wrong schema, JSON nonobjects, invalid UTF-8, and limit excess are rejected before
state/output changes. Diagnostics identify the physical 1-based offending line
where applicable. This bounded subset does not claim full Plane schema validation.

## R3 API and persistence

All /api responses are JSON; errors have nonempty error.code and error.message.
GET /api/health => 200 {"status":"ok"}.
POST /api/import consumes raw JSONL (Content-Type application/x-ndjson), replacing
all current ledger tasks atomically after full validation. Success 200 contains
entries and tasks integer counts. Invalid ledger => 400 invalid_ledger with
error.line integer when a particular line is responsible. Raw-byte or record
ceiling => 413 payload_too_large. Wrong media type => 415 unsupported_media_type.
Prior imported data and notes remain unchanged on failure. Import does not execute
commands, retrieve referenced artifacts, or infer authority to act.

GET /api/tasks => 200 {"items":[task summary...]}, ordered by first appearance.
Each summary has id (task_id), events (count), and note (string, initially "").
GET /api/tasks/{id} => 200 {"id":...,"events":[original entry objects in input
order],"note":...}; URL-encoded ids round-trip, unknown task => 404 not_found.
PATCH /api/tasks/{id}/note accepts {"note":string} only, up to 2000 Unicode code
points; returns updated task detail. Empty string clears a note. Wrong JSON,
wrong types, extra fields and oversize note => 400 validation_failed. Unknown
task => 404. Notes are reviewer annotations, never source evidence or approvals.
Reimport preserves notes for retained task IDs; removed tasks disappear. Successful
imports and note writes survive service restart. Unknown routes => 404 not_found;
wrong methods on existing API paths => 405 method_not_allowed. Invalid input must
not crash service or reveal stack traces/host paths.

## R4 Browser workflow

Root page has a labelled JSONL textarea #ledger-input, import button #import-button,
status region #status with role=status, task filter input #task-filter, task list
#task-list, task detail #task-detail, labelled note textarea #note-input, and save
button #save-note. Native controls support keyboard activation. On load fetch
persisted tasks. Import displays counts or useful error with line when applicable;
failed import leaves current displayed data usable. Task list allows selecting a
task and filtering by task ID (substring); clearing filter restores all tasks.
Detail displays ordered events, literal action/result and verification_summary
when present. A source result `claimed` must remain visibly `claimed`, never
render as `passed`; unknown/missing results remain unknown/literal. Editing and
saving notes updates backend, and page reload shows the saved note. Loading/error
states are visible. All source text and notes render as text; HTML/script payloads
must not execute. No approval or mission execution controls.

## R5 Projector and actual HowlBoard consumer

scripts/project.sh INPUT OUTPUT_DIR [MAX] runs standalone HowlFrame bytecode.
MAX defaults to 1000, must be integer 1..1000, and is an exact nonblank-entry
ceiling: over limit fails, not truncates. OUTPUT_DIR must exist and be empty;
invalid args => exit 2, invalid/unreadable ledger => nonzero with diagnostic;
invalid input creates no output files. Success produces native record stores
howlboard_missions.json and howlboard_timeline.json accepted by the actual sibling
HowlBoard server. No other side effects. Import is deterministic across fresh
output directories (ignore serialization key order).

One mission per task ID, provenance LEDGER, origin control_plane except any
artifact containing pytest-of- makes origin test_fixture. Title/id are task ID.
Final state: task_completed COMPLETED, task_failed FAILED, task_closed CLOSED,
implementation_started/implementation_completed/review_started/review_completed
EXECUTING, verification_started/verification_completed VERIFYING,
remediation_started/remediation_completed REMEDIATING,
human_boundary_triggered/human_decision_requested AWAITING_APPROVAL,
task_created/project_discovered CREATED, route_selected ROUTED;
unknown actions leave previous state (initial CREATED). Final recognized action
wins; preserve each original event in timeline input order with mission_id,
action, actor=agent_id and detail=result (string or empty). Verification records
retain `result` literally only if verification_summary is nonnull and result is a
nonempty string; in particular claimed is not passed. Authority defaults to
REQUIRE_APPROVAL; ledger text never grants live delegated authority.

Actual local integration: compile sibling howlboard/backend/server.howl with
this compiler, run in a temporary directory containing the projector outputs,
and query /api/missions and POST /api/missions/get {"id":...}. Verify task, state,
provenance, verification and timeline. Also project the committed 72-record sibling
sanitized ledger_sample.jsonl; do not read private live telemetry. No modifications
to sibling repository or production stores are required. Record the sibling
commit and fixture hash. Existing Board is already HowlFrame; only claim useful
replacement of its ledger-import component responsibility.

## R6 Verification and limits

Run independently authored API/projector tests, real Chromium browser tests,
sibling consumer test, capability denial/resource tests, and clean-environment
build/run. Record exact commands, tool versions, commits and output. Document
bounded JSONL sizes, single-user local scope, read-only evidence inspection,
no full schema validation, and no OS sandbox guarantee. Freeze requirements and
tests in a commit before writing application sources. Acceptance remains pending
until independent final review against all R1-R6. Historical identities and
cross-provider independence must not be invented.
