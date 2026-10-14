# healthwatch

HTTP health probes written in HowlFrame. Reads a JSON config of checks, runs them
sequentially with retries, evaluates status / body / JSON assertions, tracks up/down
transitions in a state file, and reports results as text or JSON. It never prints URLs,
hosts, request or response bodies, or environment values.

Specification (frozen 2026-10-13): `requirements/REQUIREMENTS.md`.

## Build and run

```
go build -o howlframe .                                    # from the repository root
./howlframe build milestone1/02-healthwatch/src/healthwatch.howl -o healthwatch.hfbc
./howlframe run --allow-caps filesystem:read=$PWD,filesystem:write=$PWD,network:fetch=api.example.com \
    healthwatch.hfbc run --state state.json checks.json
```

Runner flags go before the artifact. The runtime defaults (100000 instructions, 64 MiB) are
too small for large configs; use for example
`--max-instructions 4000000000 --max-memory-bytes 4294967296 --max-call-depth 10000`.

## Commands

```
healthwatch run      [--state FILE] [--format text|json] CONFIG
healthwatch validate CONFIG
healthwatch status   [--format text|json] [--fail-if-down] STATE
healthwatch version
```

`CONFIG` is a path or `-` for stdin. Exit codes: 0 all pass, 1 a check failed or errored,
2 usage, 65 bad config or state, 66 missing file, 70 limit (more than 500 checks),
73 write failed, 77 capability denied (also when a check was DENIED).

## Config

```json
{"version": 1,
 "defaults": {"retries": 1},
 "checks": [
  {"name": "api", "url": "https://api.example.com/health", "expect_status": [200, 204],
   "body_contains": "ok",
   "json_assert": [{"path": "status", "equals": "ok"}, {"path": "items.0.id", "exists": true}]},
  {"name": "echo", "url": "https://api.example.com/echo?t=${ENV:API_TOKEN}", "method": "POST", "body": "ping"}
 ]}
```

Config problems are reported as `healthwatch: config: <path>: <message>` (one error, fixed
precedence). `${ENV:NAME}` is allowed in `url` and `body` only.

## Grants

| Operation | Grant |
|---|---|
| read CONFIG / STATE | `filesystem:read=<dir>` |
| write STATE | `filesystem:write=<dir>` (checked before any request is sent) |
| each target host | `network:fetch=host:port` (the coarse `network` grant does not authorize fetching) |
| `${ENV:}` | `environment` |
| CONFIG `-`, `validate` on stdin, `version`, `help` | none |

A check whose host is not granted (including a redirect hop to one) is reported `DENIED`; the
run continues and exits 77 at the end. The tool never needs `process` or `database`.

## Tests

* Frozen black-box suite (local stdlib HTTP servers, no external network):
  `HF_BIN=... APP_ARTIFACT=... python3 tests/acceptance/run_acceptance.py tests/acceptance/launcher.example.sh`
* Clean-environment reproduction from the committed tree: `scripts/clean_repro.sh`.

## Known limitations

* Numbers are classified through JSON re-encoding, so `1.0` and `1e2` count as integers.
* `since`/`last_run` are whole seconds from the runtime clock (`time_now`).
* A check that has not finished when the runner `--deadline` expires stops the whole run;
  the state file is only written at the end, so it stays unchanged.
* The runtime exposes no request timeout; only the runner deadline bounds a slow server.
