# taskflow

A declarative task runner written in HowlFrame. A *Flowfile* defines variables and named
tasks with dependencies; taskflow resolves the graph deterministically (rejecting cycles and
unknown tasks), expands variables **after** splitting command lines into words (an inserted
value never changes how a line is split, and nothing is ever interpreted by a shell), and
runs each command through the runtime's governed `exec`.

Specification (frozen 2026-10-13): `requirements/REQUIREMENTS.md`.

## Build and run

```
go build -o howlframe .                                   # from the repository root
./howlframe build milestone1/03-taskflow/src/taskflow.howl -o taskflow.hfbc
./howlframe run --allow-caps "filesystem:read=$PWD,process:exec=/usr/bin/python3.14 **" \
    taskflow.hfbc run -f Flowfile build
```

Programs run only if granted: `process:exec=<canonical absolute program path> <argv patterns>`
(a symlinked path is refused; grant the resolved path). Children start with an empty
environment except for `process:env=NAME` grants. Runner flags such as `--max-exec-output-bytes`
and `--deadline` go before the artifact; for large Flowfiles also raise `--max-instructions`,
`--max-memory-bytes` and `--max-call-depth` above the small defaults.

## Flowfile

```
set GREETING = hello world

task default after build test
task build
  desc compile
  run /usr/bin/python3.14 -c "print('${GREETING}')"
task test after build
  run /usr/bin/python3.14 -c 'print(1)'
```

Column 0: `set NAME = VALUE`, `task NAME [after DEP...]`. Indented: `desc TEXT`, `run PROGRAM [ARG...]`.
Quoting is `"..."` (escapes `\"` and `\\`), `'...'` (literal) and adjacency. `${NAME}`,
`${env:NAME}` and `$$` are the only substitutions; `-D NAME=VALUE` overrides `set` and its value
is used literally.

## Commands

```
taskflow run     [-f FILE] [-D NAME=VALUE]... [--dry-run] [--keep-going] [--report FILE] [--history FILE] [TASK...]
taskflow list    [-f FILE]
taskflow plan    [-f FILE] [TASK...]
taskflow history [--last N] FILE
taskflow version
```

Exit codes: 0 ok, 1 a task failed, 2 usage, 65 Flowfile/history error, 66 missing file, 70 limit,
73 write failed, 77 capability denied (outranks 1).

## Grants

| Operation | Grant |
|---|---|
| read the Flowfile, `--history` input, `history FILE` | `filesystem:read=<dir>` |
| write `--report` / `--history` | `filesystem:write=<dir>` (checked before any command runs) |
| each program | `process:exec=<program> <patterns>` |
| child environment | `process:env=NAME` |
| `${env:NAME}` | `environment` (never needed by `list`, `plan`, `--dry-run`) |

## Tests

* Frozen black-box suite: `HF_BIN=... APP_ARTIFACT=... python3 tests/acceptance/run_acceptance.py tests/acceptance/launcher.example.sh`
* Clean-environment reproduction from the committed tree: `scripts/clean_repro.sh`.

## Runtime support this app relies on

Two small runtime additions made for this milestone, with regression tests in `internal/vm`:
`(exec program list)` for a run-time argument vector, and the exit code plus output of a failed
child being visible to `try_let/catch`. See `docs/reference/process_authority.md`.

## Known limitations

* Recursion depth: planning and cycle detection recurse once per dependency level, so a chain
  of roughly 1000 tasks needs `--max-call-depth` above the runtime default of 1000.
* Child output with invalid UTF-8 is shown with replacement characters (the runtime reports a
  failed child's output through JSON); output of successful children is byte-exact.
* The command count in a report counts started commands: a denied command is not counted.
* `-D` values are inserted literally (they are not scanned for `${...}`).
* The source contains no non-ASCII string literals (the lexer double-encodes them).
