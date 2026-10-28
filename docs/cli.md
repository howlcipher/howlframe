# Command Line Interface (CLI)

HowlFrame provides a clean, subcommand-based CLI for the core compilation and execution workflows.

## `howlframe check`
Usage: `howlframe check [--profile=default|governed] <source.howl>`

Parses, type-checks, and verifies the `.howl` source file without emitting an artifact. It ensures that the code follows the syntax and structural rules. It will check the syntax, resolve modules, apply type-checking, and run the target-independent HFIR verification gate.

If successful, it prints `OK: <source.howl>` and exits with status 0. Otherwise, it prints structured diagnostics and exits with a non-zero status. `check` and `build` write source diagnostics to **standard output** and exit with status 1: a parser or checker error is one JSON object, `{"reason":"undefined reference to function \"nosuch\"","line":1,"column":23}`, and `build` reports a construct the target cannot compile as a JSON array of objects with `code` (such as `HFIR_TARGET_INFEASIBLE`), `severity`, `message` and `location`. `check` is target-independent and does not run that feasibility check, so a program can pass `check` and still fail `build`; build for the target to confirm. Usage and file errors (a missing or unreadable source file, an unknown flag) go to standard error. Runtime failures from `run` go to standard error, for example `{"phase":"runtime","code":"CONVERSION_ERROR","function":"main","instruction":0,"opcode":"CONVERT","message":"cannot convert \"abc\" to float"}`.

## `howlframe build`
Usage: `howlframe build <source.howl> [-o <output>] [--target=bytecode|go|js|wasm] [--profile=default|governed]`

Compiles a `.howl` source file to a standalone HowlFrame bytecode artifact (`.hfbc`) by default. Without `-o`, it writes `<basename>.hfbc` to the **current directory**, not next to the source. `--target go` writes `server.go` (and `server_test.go` when the program has tests), and `--target js` writes `app.js` (and `app.test.js`), into the current directory or the directory given with `-o`; an `-o` path ending in `.go` or `.js` writes only that file.

## `howlframe run`
Usage: `howlframe run [options] <artifact.hfbc | source.howl> [arguments...]`

Executes a compiled HowlFrame bytecode artifact, or compiles and runs a `.howl` source file. Options go before the artifact. Every argument after the artifact is passed to the program verbatim as `(cli_args)`, including a literal `--` (the built-in usage text shows `[-- arguments...]`, but `--` is not consumed).

**Important**: Capabilities are denied by default. You must explicitly grant capabilities to the runtime.

### Options
* `--allow-caps` : Comma-separated capabilities to allow (e.g., `network,filesystem,process,environment,database`). `network` serves HTTP and calls the model forms; outbound `fetch`/`http_request` needs one `network:fetch=<host>[:<port>]` per host (exact match, see `docs/reference/network_authority.md`). Repeatable `filesystem:read=<root>` grants permit reads; `filesystem:write=<root>` grants permit writes and `mkdir`, without granting reads. `filesystem` remains unrestricted. Roots are cleaned and anchored to the runner cwd at parse time. Targets must remain within a matching root after lexical normalization and symlink resolution. `file://` stores require `database` plus read coverage for open/get/keys and both read/write coverage for put/delete. Empty roots, unknown sub-keys, and scoped forms of other capabilities are unknown-capability errors. Denials occur before I/O with `CAPABILITY_DENIED`. Concurrent symlink changes remain a TOCTOU caveat. Generated Go/JavaScript remain coarse for filesystem. `process` alone never authorizes `exec`: each program needs `process:exec=<absolute path>[ <pattern>...]` (exact argv; `*` one non-empty argument not starting with `-`, `\*` a literal `*`, trailing `**` the rest; no PATH lookup; symlinked programs refused), on every path including generated Go/JavaScript. exec children get an empty environment except `process:env=NAME` grants. `spawn_agent`, `spawn` and route handlers receive only the `(grants ...)` they are handed (route handlers also keep `network`), never more than the parent. See `docs/reference/process_authority.md`.
* `--max-instructions` : A finite instruction ceiling (default `100000`). Once the ceiling is reached, execution halts to prevent infinite loops and runaway resource consumption.
* `--max-call-depth` : Positive CALL recursion and SPAWN_AGENT nesting ceiling (default `1000`).
* `--max-startup-instructions` : `run` only. A finite instruction ceiling for the startup code of a program that defines an HTTP server: everything that runs before the server begins serving, such as validating persisted state, whose cost grows with the stored data. Request handlers always use `--max-instructions`, each with a fresh budget. Default `10000000` for server programs when `--max-instructions` is not given; if `--max-instructions` is given explicitly it governs startup too unless this flag is also given. Programs that do not define an HTTP server are unaffected. The larger of the two ceilings applies to startup.
* `--max-memory-bytes` : Positive cumulative allocation charge ceiling (default `67108864`, 64 MiB). Charges approximate storage; they are not a measured live heap limit and are never reclaimed. For an HTTP server the ceiling applies to each request separately (a server would otherwise exhaust one process-wide budget); the startup code and every other program use one budget for the whole run.
* `--max-fetch-bytes` : Positive fetch response body ceiling (default `10485760`, 10 MiB).
* `--max-exec-output-bytes` : Positive combined stdout/stderr ceiling for exec (default `10485760`, 10 MiB).
* `--max-request-bytes` : Positive ceiling in bytes for an HTTP route's request body (default `1048576`, 1 MiB). Reading a larger body with `(req_body req)` or `parse_json` over `req.body` raises a catchable `LIMIT_EXCEEDED`, so an application can answer `413`. Zero, negative and non-numeric values exit with status 2.
* HTTP servers started by `run` always apply these connection timeouts: 10 s to read request headers, 30 s to read the whole request (serial handlers receive a read error if they consume a truncated/stalled upload; partial bytes are never supplied as complete JSON), 120 s idle. A handler may run longer than that; only reading the request is bounded.
* `--listen-host` : HTTP listen IP literal, such as `127.0.0.1` or `::1`. The default empty value binds all interfaces. Hostnames and host:port strings are rejected with exit 2. This runner-owned restriction also applies to `port_available`.
* `--access-log` : Write one line per HTTP request to stderr: `access method=GET path=/v1/x status=200 duration_ms=3 request_id=<id>`. The path is the request path as sent (percent-encoded, no query string). Off by default. A handler fault always writes a `fault request_id=<id> method=<M> path=<P> error=<text>` line, with or without this flag. See `docs/guides/http_api.md`.
* `--timing-log` : Diagnostic only, off by default, and it does not change behaviour. It writes one line per HTTP request to stderr, separate from the access line (whose format is unchanged): `timing request_id=<id> method=GET path=/v1/x start_unix_ns=<wall clock> start_mono_ms=<ms> locked_mono_ms=<ms> end_mono_ms=<ms> wait_ms=<ms> handler_ms=<ms> total_ms=<ms>`. The `*_mono_ms` values are monotonic milliseconds since the process started. `locked_mono_ms` is when the route's serial lock was acquired, so `wait_ms` is time spent queued and `handler_ms` is time inside the handler. When a persisted-store write takes 100 ms or more, it also writes `store_timing kind=put|delete bytes=<n> marshal_ms= write_ms= fsync_ms= close_ms= rename_ms= dirsync_ms= total_ms= mono_ms=`. To trace Go garbage collection at the same time, start the runner with `GODEBUG=gctrace=1` in its environment (for example `GODEBUG=gctrace=1 howlframe run --timing-log ...`). The Go runtime then writes one `gc N @<s>s ...` line per collection to stderr, and the `@<s>s` offset can be matched against `*_mono_ms`. This is a Go runtime variable, not a HowlFrame flag. It applies only to the runner process and is never passed to child processes (the process capability strips `GODEBUG`).
* `--deadline` : Optional positive duration such as `5s` or `100ms`; absent means no wall-clock deadline. Explicit `0s`, negative durations, and invalid durations are rejected. Cancels bytecode fetch, exec, sleep, and model requests and checks between instructions.

* `--receipt <path>` : Optional runner-written `howlframe.receipt/v0` JSON file,
  supported by `run --target bytecode`/`bc` for artifacts and `.howl` source,
  and by legacy `-run-bc --receipt <path> <artifact>`. Interpreter targets
  reject this option. Absent or empty means no receipt and unchanged output.

The receipt records the exact artifact file SHA-256; for source compiled in
process it hashes canonical `bytecode.WriteArtifact` bytes. HFBC carries a
format version but no compiler identity, so `compiler_version` is the runner's
HowlFrame Version. It includes sorted unique grants, configured limits, executed
instructions (including synchronous SPAWN_AGENT work), bounded capability
decisions, and exit status/error. JSON uses fixed field order, two-space indent,
and a trailing newline; grant/effects are always arrays. No fields are omitted.
A nonzero exit/return has status `error` with `error: null`; runtime traps have
code 1 and a structured error. Zero deadline_ms means none (positive durations are
rounded up to whole milliseconds).

Targets exclude URL paths/query/fragment/userinfo, env values, command arguments,
DSNs, SQL, store keys/values, response bodies, and model prompts. Filesystem
paths are recorded as given; resource names may still be sensitive. Summaries
are bounded to 256 bytes. Host failure messages are replaced by a code-only
message, except safe limit/capability messages; stderr diagnostics are unchanged.
Ambient print/stderr/sleep/time/read_line/exit are omitted. The first 10,000
capability decisions are retained; `effects_truncated` reports overflow. Children
share the recorder; detached SPAWN effects after finalization are dropped.

The runner writes after execution and before exit using a same-directory temp
file (0600 permissions) and rename. Program stdout/stderr are unchanged and
cannot forge the receipt through print. If writing fails, a diagnostic goes to
stderr: successful execution becomes exit 1; an existing failure code is kept.
Parent directories must exist. Receipts have no signature or attestation and
are independent of sealed ExecutionEvidence. See the
[C5 journal](journals/2026-10-06_c5_execution_receipt.md).

These limits also apply to legacy `-run-bc`. The new flags apply to `run` targets `bytecode`/`bc`; AST interpreter memory and deadline behavior is unchanged. Exceeded ceilings produce structured `LIMIT_EXCEEDED`. Blocking `read_line` and HTTP serving are not cancelled by the deadline; print output remains uncapped.


### Sizing budgets for large inputs

The defaults suit small programs. A program that processes a large file or many records usually needs both ceilings raised, and the runner chooses them, not the program:

```bash
howlframe run --allow-caps filesystem:read=./data \
  --max-instructions 500000000 --max-memory-bytes 4294967296 app.hfbc data/input.log
```

* **Instructions.** Every executed instruction counts, including loop bookkeeping: `(while (< i n) (set i (+ i 1)))` costs 9 instructions per iteration. Run once on a representative input with a generous ceiling and `--receipt r.json`; the receipt's `instructions_used` is the measured cost, and a ceiling a small multiple above it keeps the bound without failing on normal variation.
* **Memory.** `--max-memory-bytes` caps cumulative allocation charges, not live memory. A value is charged when an instruction creates it (for example string concatenation, each `str_split` part, `list`, `dict` and `append` slots, file and process output), and the charge is not returned when the value becomes garbage. Output of `read_file`, `exec` and `fetch` is charged one byte per byte, so `read_file` of a 40 MB file is charged about 40 MB. `list_len` and `for` read those bytes without charging anything more, and `bytes_to_string` charges the string it makes, one byte per byte. `append` charges 8 bytes per element of the new list on every call, so appending n items one at a time charges about 4n² bytes. `str_split`, `str_len` and `str_slice` charge every part they split the text into. Size the ceiling from everything a run creates, not from what it keeps.
* Exceeding either ceiling stops the program with exit status 1 and writes the usual runtime error object to standard error, with `"code":"LIMIT_EXCEEDED"` and `"message"` set to `"instruction limit exceeded"` or `"memory limit exceeded (max N bytes)"`.

## `howlframe inspect`
Usage: `howlframe inspect --caps <artifact.hfbc>`

Reports, without executing anything, the capabilities a compiled artifact requires and the instruction sites that need them, as compact JSON, for example `{"capabilities":["filesystem"],"sites":[{"function":"main","index":1,"opcode":"READ_FILE","capability":"filesystem","reason":"opcode registry"}]}`. It reads `.hfbc` artifacts only; build source first.

## `howlframe version`
Usage: `howlframe version`

Prints the current version of the HowlFrame CLI and the supported HFBC artifact format version.

---

## Legacy Compatibility Interface
The CLI retains full backward compatibility with the original flat flag structure:

* `-validate` : Run lexer, parser, semantic checker, and AOT HFIR verifier without transpiling.
* `-compile-bc` : Compile AST to a bytecode artifact (binary HFBC) (production bytecode compiler, gated by AOT HFIR semantic verifier; HOWL-CANON-010). Emission stays `bytecode.CompileToBytecode`. The promote, defer, and kill checklist is `docs/reference/lowered_hfir_prod_flip_criteria.md`. #90 stays Partial.
* `-compile-hfir-bc` : Compile directly from semantic HFIR to a bytecode artifact (experimental research direct lowering pathway; HOWL-CANON-010). This flag stays the dogfood path until the Owner authorizes a flip PR.
* `-run-bc` : Run a bytecode artifact.
* `-compile-wasm` : Compile typed SSA/CFG to WebAssembly Text.
* `-run` : Interpret and execute a script directly.
* `-mask-plan` : Print the deterministic constrained-decoding mask plan.
* `-optimization-plan` : Print the deterministic compile-time optimization plan.

These flags are not part of the primary onboarding path but remain fully supported for existing integrations.

## JavaScript `web_app` generation

`howlframe build` defaults to standalone HFBC. Select JavaScript explicitly:

```bash
howlframe build frontend.howl --target js -o build/app.js
```

The compatibility interface `howlframe -o build frontend.howl` also writes
`build/app.js` and any generated native tests. JavaScript output is not HFBC and
is not executed by `howlframe run`.

The JavaScript target accepts `(fetch url method body headers)` where `headers`
is a dictionary, in addition to the existing two/three-argument forms. Destination
grants and redirect checks still apply. The optional headers operand is rejected
explicitly on unsupported bytecode, interpreter and generated-Go targets; it is
never silently dropped. `(url_encode text)` is a JavaScript-target expression
using component encoding; it is useful for query values and does not make browser
path normalization disappear. Unsupported targets fail closed.

`(granted "filesystem:write=/path")` and the corresponding `read=` form query
the same canonical filesystem containment check used by actual I/O, without
creating a probe file. Coarse capability-family queries retain their previous
behavior. This grants no new authority and allows multi-output CLIs to detect
all permission denials before writing either destination.

## `howlframe interop`
Usage: `howlframe interop --json`

Prints the `howl.interop/v1` descriptor other Howl components read to check
that the commands and flags they use still exist (see `docs/INTEROP.md` in the
`howl` repository). It lists each subcommand with its flags, the legacy
single-dash flags under the empty path, the HFBC format, and the exit codes:
0 ok (under `run`, the program's own exit status passes through), 1 error, and
2 for an invalid resource flag.
