# EvidenceDesk

Import a local HowlPlane evidence-ledger slice, inspect ordered source events,
and keep persistent reviewer annotations. A source claim stays a claim: this
application neither approves work nor executes missions.

The backend and browser interactions are written in HowlFrame. The HTML/CSS
shell is handwritten; `build.sh` generates browser JavaScript from `frontend.howl`
and standalone VM bytecode from the backend and projector sources.

From the repository root:

```bash
bash milestone3/evidencedesk/scripts/build.sh
bash milestone3/evidencedesk/scripts/run.sh
```

Open `http://127.0.0.1:8093`, paste JSONL into the labelled input, import, select
and filter tasks, then save an annotation. Reloading restores imported tasks
and notes. Invalid imports preserve the prior data. The API enforces 256 KiB,
1000 records and 2000 Unicode code points per note.

`HOWLFRAME_BIN` selects an installed compiler. `EVIDENCEDESK_PORT` changes the
port, `EVIDENCEDESK_DATA` selects the persistent native store, and
`EVIDENCEDESK_STATIC` selects built assets. The data parent directory must exist.
The runner binds exactly `127.0.0.1` and grants only serving, environment,
database, the chosen state file and static asset directory. The script supplies
a finite 10-million-instruction budget. No process, outbound-fetch or model
grant is needed. These are runtime capability checks, not OS sandboxing.

The canonical task API is `GET /api/tasks/{id}` and
`PATCH /api/tasks/{id}/note`. Browser clients use the companion
`GET/PATCH /api/task?id=...` to preserve accepted IDs such as `.` and `..`
that URL implementations normalize in path segments. The compiled frontend
uses the JavaScript target's `url_encode` expression and explicit fetch headers.

To project a ledger for HowlBoard, create an empty output directory and run:

```bash
bash milestone3/evidencedesk/scripts/project.sh INPUT.jsonl OUTPUT_DIR 1000
```

The projector validates the entire input before writing either native store.
The optional maximum is an exact limit: excess input fails rather than truncates.
`app/ledger.howl` is also usable on its own: `parse_within` applies the same
validation under a caller-chosen byte ceiling (`parse` keeps 262144 bytes).
It replaces the ledger-import responsibility of an existing HowlFrame component;
this is not a claim that HowlBoard has newly adopted the language. The actual
sibling consumer needs the transparent, temporary capability-declaration adapter
in [the R5 clarification](requirements/R5_COMPATIBILITY_ADDENDUM.md).

Verification:

```bash
HOWLFRAME_BIN=/absolute/path/to/howlframe \
  python3 milestone3/evidencedesk/tests/verify_workflows.py
```

This runs the six independently authored API/projector tests, restart and
encoded-ID checks, real Chromium interaction and XSS tests, the actual sibling
Board consumer with both sanitized fixtures, and capability/resource denials.
It requires Python Playwright and an installed Chromium; `CHROMIUM_EXECUTABLE`
can select the browser and `HOWLBOARD_REPO` can select the sibling checkout.
Only the committed sanitized fixture is read. No private live telemetry or
production stores are needed. The initial unadapted consumer failure is retained
in `evidence/board_unadapted_failure.txt`.

Limits: single-user local operation, whole-state persistence, bounded input,
and a documented subset of the evidence schema. Startup configuration errors
may use generic runtime diagnostics. Projection validation is atomic; an OS
failure during publication of its two separate output files can leave one file,
so use a fresh temporary output directory before replacing live consumer data.
