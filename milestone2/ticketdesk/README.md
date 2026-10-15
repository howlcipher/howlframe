# ticketdesk

A small issue-tracker HTTP API written in HowlFrame and run on the standalone VM: bearer-token
authentication with roles (`admin`, `member`, `viewer`), ownership rules, strict JSON validation, one error
shape with stable codes, durable file-backed state and request-level observability.

Contract (frozen 2026-10-13): `requirements/REQUIREMENTS.md`. Guide: `docs/guides/http_api.md`.

## Build and run

```
go build -o howlframe .                                    # from the repository root
./howlframe build milestone2/ticketdesk/src/ticketdesk.howl -o ticketdesk.hfbc
export TICKETDESK_ADMIN_TOKEN=<16 to 256 printable characters>   # the bootstrap admin token
export TICKETDESK_DATA=/var/lib/ticketdesk/state.json            # directory must exist
./howlframe run --allow-caps "network,database,environment,filesystem:read=/var/lib/ticketdesk,filesystem:write=/var/lib/ticketdesk" \
    --max-request-bytes 1048576 --access-log ticketdesk.hfbc
```

`TICKETDESK_PORT` (default 8080) selects the port. Without a runner `--max-memory-bytes` large enough for the
expected load the default (64 MiB) applies per request. A missing grant stops the server at startup with
`ticketdesk: capability denied: <name>` and exit status 77; invalid environment gives status 2, an unusable
data file 65, a missing data directory 66.

## Try it

```
curl -s localhost:8080/v1/health
curl -s -X POST localhost:8080/v1/tokens -H "Authorization: Bearer $TICKETDESK_ADMIN_TOKEN" \
     -H 'Content-Type: application/json' -d '{"name":"ci","role":"member"}'      # the secret is shown once
curl -s -X POST localhost:8080/v1/tickets -H "Authorization: Bearer <token>" \
     -H 'Content-Type: application/json' -d '{"title":"Printer on fire","priority":"high"}'
curl -s 'localhost:8080/v1/tickets?status=open&limit=10' -H "Authorization: Bearer <token>"
```

## Tests

* Frozen black-box suite (101 tests): `HF_BIN=... APP_ARTIFACT=... python3 tests/acceptance/run_acceptance.py tests/acceptance/launcher.example.sh`
* Developer-experience checker: `HF_BIN=... python3 dx/check_dx.py`; guide rule: `go test -run HTTPAPIGuideRule .`
* Everything from a clean checkout of the committed tree: `milestone2/scripts/clean_repro.sh`.

## Design notes

* Handlers run one at a time (`(serialize_requests)`), so every read-modify-write is atomic. The state file is a
  native store replaced atomically (synced temporary file and rename) after every mutation.
* Token secrets are `td_` plus 64 random hex characters; only their SHA-256 is stored, compared in constant
  time. The bootstrap token is never written to disk.
* `owner` of a ticket and the cursor of a listing are positions in a monotonic counter, never reused.

## Known limitations

* The server is single-process and serializes requests: it is a functional backend, not a throughput benchmark.
* Listing reads every ticket key and sorts in the language (quadratic); suitable for thousands of tickets, not millions.
* TLS, rate limiting and token expiry are out of scope (PLAN.md section 6).
