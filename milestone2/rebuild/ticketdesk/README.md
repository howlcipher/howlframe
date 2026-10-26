# Fresh Ticketdesk implementation

This is a new requirements-first implementation of the original frozen
Ticketdesk contract. Historical code and review findings remain under
`milestone2/ticketdesk/`; this candidate does not inherit their acceptance.

Build from the repository root:

```bash
go build -o /tmp/howlframe .
/tmp/howlframe build milestone2/rebuild/ticketdesk/src/ticketdesk.howl \
  -o /tmp/ticketdesk.hfbc
```

Set `TICKETDESK_ADMIN_TOKEN`, `TICKETDESK_DATA` (an absolute state file in an
existing directory), and optionally `TICKETDESK_PORT` (8080 by default). Run:

```bash
/tmp/howlframe run \
  --allow-caps "network,database,environment,filesystem:read=$DATA_DIR,filesystem:write=$DATA_DIR" \
  --max-request-bytes 1048576 --access-log /tmp/ticketdesk.hfbc
```

The original user workflow and API are documented in
[the original guide](../../ticketdesk/README.md). These commands use default
request ceilings. Full startup validation scales with stored records, so larger
states can require an explicit higher `--max-instructions` and memory budget.
The server serializes requests. Whole-store atomic replacement limits throughput.

Unlike the previous implementation, each bucket publishes ticket records and
their order in one atomic store write. Creates reserve an ID first; a crash can
burn an unacknowledged ID but cannot leave a ticket outside its listing index.
Tokens, their order and revocations also publish as one atomic collection.
Authentication scans stored token hashes using constant-time comparisons, trading
indexed lookup speed for a simpler crash-consistency boundary. Secrets and
bootstrap credentials are never persisted. Existing data is validated before
startup writes and before achievable grant/port rejections.

The state format is explicitly version 2. Old files are rejected with exit 65
instead of silently listing no tickets. Stop the old server, then convert offline:

```bash
python3 milestone2/rebuild/ticketdesk/migrate_state.py OLD.json NEW.json
```

The Python standard-library maintenance utility leaves the source unchanged,
recovers canonical legacy records even when old indexes are incomplete, and
atomically publishes a new mode-0600 destination without overwriting an existing
file. Point `TICKETDESK_DATA` at the new file only after reviewing the conversion.
It is not an application runtime dependency.

The frozen 101 tests and 13 DX checks remain authoritative. Additional retained
probes are in `tests/run_reassessment.py`, including default budgets, real crash
durability, failed-startup preservation, invalid states, truncated framing and
migration. The probe/close/bind port race remains a documented non-blocking
limitation. The runtime is capability-governed, not an OS sandbox.
