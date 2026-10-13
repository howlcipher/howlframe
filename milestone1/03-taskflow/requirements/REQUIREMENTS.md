# REQUIREMENTS: taskflow (Milestone 1, application 3 of 3)

> **STATUS: FROZEN, dated 2026-10-13.** Changes require a separately approved requirements revision (see 1.5). Tool name: `taskflow`. Domain: build/automation orchestration (a task runner with dependency resolution and governed process execution). Unrelated to the Howl ecosystem.

## 0. Problem statement

Developers and operators keep small automation pipelines ("generate, build, test, package") as ad-hoc shell scripts that cannot express dependencies, hide quoting and injection bugs, and run with the full authority of the user. `taskflow` reads a declarative text file (the *Flowfile*) that defines variables and named tasks with dependencies and command lines, resolves the dependency graph deterministically (rejecting cycles and unknown tasks), expands variables safely (an expanded value is never re-split or re-interpreted: **no shell, no word splitting, no injection**), executes each command directly as an argv vector through the governed process capability, reports per-task results, optionally writes a machine-readable report and a bounded run history, and fails cleanly and early when it lacks authority (unlisted programs, environment reads, file writes). It is the hardest of the three Milestone 1 applications: it combines a small language (parser, quoting, variable substitution), graph algorithms, process execution with output capture, failure policies, persistence, and the strictest capability behavior.

## 1. Common conventions (apply to every Milestone 1 application)

### 1.1 Deliverable layout and build

The implementation lives under `milestone1/03-taskflow/src/` (`.howl` source; HFIR is permitted if it qualifies under directive D24-B). The program MUST be built to a standalone HowlFrame artifact:

```
howlframe build milestone1/03-taskflow/src/taskflow.howl -o milestone1/03-taskflow/build/taskflow.hfbc
```

The artifact MUST execute on the standalone VM only (`howlframe run`, or legacy `-run-bc` with an equivalent `-compile-bc` artifact). It MUST NOT depend on generated Go/Python/JavaScript, on helper scripts, or on any external program at run time unless this document says so.

### 1.2 Invocation convention ("the launcher")

All behavior is specified for the invocation `taskflow <args...>`, which the acceptance suite performs through a **launcher**: an executable file with this contract:

```
LAUNCHER GRANTS [APP_ARG...]
```

* `GRANTS` is exactly ONE argument: the comma-separated runtime grant list for `--allow-caps` (for example `filesystem:read=/tmp/x,filesystem:write=/tmp/x`). The empty string means "no grants": the launcher MUST then pass no `--allow-caps` at all.
* `APP_ARG...` are passed to the application verbatim, as its `cli_args`, in order, with no re-quoting and no interpretation of flag-looking arguments.
* The environment variable `HF_RUN_FLAGS` (whitespace-separated runner flags such as `--max-instructions 5000`, `--max-fetch-bytes 1024`, `--deadline 1s`, `--max-exec-output-bytes 100`) MUST be inserted by the launcher as runner options before the artifact path. It is the only channel for resource limits.
* The launcher forwards its stdin, stdout, stderr, working directory and process environment unchanged and exits with the application's exit status.

Reference launcher (`tests/acceptance/launcher.example.sh`):

```
howlframe run [--allow-caps "$GRANTS"] $HF_RUN_FLAGS milestone1/03-taskflow/build/taskflow.hfbc "$@"
```

Unless a test says otherwise the acceptance suite sets `HF_RUN_FLAGS="--max-instructions 4000000000 --max-memory-bytes 4294967296 --max-call-depth 10000"` (generous but finite ceilings; the runtime defaults of 100000 instructions / 64 MiB / call depth 1000 are far too small for realistic input and are NOT an application requirement). Resource-limit tests substitute smaller values.

Grant syntax (see `docs/cli.md`, `docs/reference/network_authority.md`, `docs/reference/process_authority.md`): `filesystem:read=<abs dir>`, `filesystem:write=<abs dir>`, `network:fetch=<host>:<port>`, `process:exec=<abs program> <pattern...>`, `process:env=<NAME>`, `environment`. The application never receives more than the grants it is launched with and MUST NOT attempt to widen them.

### 1.3 Streams and exit codes

* Results go to **stdout**. Diagnostics go to **stderr**; every diagnostic line MUST start with `taskflow: `. Because the runtime's `stderr` built-in adds no newline, the application MUST terminate each diagnostic line with `\n`.
* stdout and stderr are UTF-8; text is passed through byte-exact (no re-encoding, no case-folding of non-ASCII).
* Exit codes (all applications):

| Code | Meaning |
|---|---|
| 0 | Success, nothing to report as a finding |
| 1 | Success of the tool, but the analysis found failures/violations (defined per application) |
| 2 | Usage error (bad command, bad/missing/duplicate option, wrong operand count). Nothing else is done: no input read, no file written |
| 65 | Input data/format/config error (data is unusable) |
| 66 | A required input file does not exist or is not a regular readable file |
| 70 | A resource limit was reached (application-level limit or a runtime `LIMIT_EXCEEDED` the application is able to intercept) |
| 73 | An output file cannot be written (and the failure is not a capability denial) |
| 77 | Capability denied by the runtime (see 1.4) |

### 1.4 Capability denial behavior (uniform)

When the runtime refuses an operation with `CAPABILITY_DENIED` the application MUST:

1. stop the operation that needed it, with no partial side effects (no partially written output file; nothing destructive done);
2. print exactly one diagnostic line of the form `taskflow: capability denied: <capability-name>[ (<what>)]` on stderr, where `<capability-name>` is one of `filesystem`, `network`, `process`, `environment`, `database`;
3. exit with status 77 (unless this document defines a different, explicitly stated, per-item handling);
4. NEVER print raw runtime error objects (strings containing `"phase":`), host stack traces, or the text of secret values.

Missing files are NOT denials (exit 66, or as specified). Operations that do not need a capability MUST work with an empty grant list.

### 1.5 Freeze notice

**FROZEN 2026-10-13.** This document, together with `ACCEPTANCE.md` and `tests/acceptance/`, was written by the independent selector/evaluator before any implementation. Changes require a separately approved requirements revision that is recorded as such (new revision header, dated, with rationale); implementers MUST NOT edit, dilute or remove requirements or tests to claim success. The freeze is sha-agnostic: it binds to the content as authored, not to a repository revision.

## 2. Command-line interface

```
taskflow run     [-f FILE] [-D NAME=VALUE]... [--dry-run] [--keep-going] [--report FILE] [--history FILE] [TASK...]
taskflow list    [-f FILE]
taskflow plan    [-f FILE] [TASK...]
taskflow history [--last N] FILE
taskflow version
```

* `-f FILE` selects the Flowfile (default `Flowfile`, resolved against the current working directory); `-f -` reads the Flowfile from standard input. Options take their value as the NEXT argument (`-f x`, never `-fx` or `--file=x`) and may appear before or after the operands (`TASK...`, `FILE`). `-D` may be repeated (a later `-D` of the same NAME wins); every other option at most once. `NAME` of `-D` is `[A-Za-z_][A-Za-z0-9_]*`; `VALUE` may be empty and may contain `=`. Unknown options, options not valid for the command, a missing option value, a malformed `-D`, a non-positive-integer `--last`, wrong operand counts (`history` takes exactly one FILE; `list` and `version` none; `plan` and `run` any number of task names) are usage errors: exit 2, nothing on stdout, no I/O of any kind.
* `TASK...` are task names to run/plan, in the given order. If none is given the task named `default` is used; if there is no `default` task that is exit 2 (`taskflow: no task given and no 'default' task`). An unknown task name is exit 2 (`taskflow: unknown task: NAME`), detected after the Flowfile was successfully loaded.
* `help`, `--help`, `-h` print a usage text to stdout (first line begins `usage: taskflow`), exit 0. `version` prints exactly `taskflow 1.0.0\n`, needs no grants. No command or unknown command: exit 2.
* Exit codes are those of 1.3. For `run`: after execution, **77** if any capability was denied (a command, or the writing of `--report`/`--history`), else **1** if any task failed, else **0**. `list`, `plan`, `history`: 0 on success.
* Programs are executed with the runner's current working directory and an environment that contains ONLY variables explicitly granted to the runtime with `process:env=NAME` grants (nothing is inherited).

## 3. The Flowfile

UTF-8 text, lines end in `\n` or `\r\n`. Maximum 1000 tasks (more => exit 70, `taskflow: task limit exceeded (max 1000)`). A line is **indented** if its first character is a space or a tab, otherwise it starts at **column 0**. Blank lines and lines whose first non-blank character is `#` are ignored (anywhere). There are no inline comments (a `#` later in a line is ordinary text).

Column-0 directives:

* `set NAME = VALUE`: defines a variable. `NAME` is `[A-Za-z_][A-Za-z0-9_]*`; the separator is the first `=`, spaces/tabs around it are ignored; `VALUE` is the rest of the line with leading/trailing blanks removed (may be empty, may contain `=` and quotes literally). A name defined twice is an error.
* `task NAME` or `task NAME after DEP...`: starts a task. `NAME` is `[A-Za-z0-9_.-]+`; `after` is followed by one or more dependency names (whitespace-separated, each a valid task name); nothing else may follow. A task name defined twice is an error. A dependency that names no task is an error (reported at the `task` line of the dependent). A dependency cycle anywhere in the file is an error (even among tasks nobody asks for), reported as `... dependency cycle: a -> b -> a` listing the cycle's task names in dependency order; the line number is that of the `task` line of the cycle member that appears first in the file.

Indented directives (belong to the most recent `task`; an indented directive before any task is an error):

* `desc TEXT`: one-line description (rest of the line trimmed; at most once per task).
* `run PROGRAM [ARG...]`: one command. At least a program is required. A task may have any number (including zero) of `run` lines, executed in order.

Anything else (unknown column-0 word, unknown indented directive, malformed line) is an error. **Load errors** are exit 65 with `taskflow: <FILE>:<LINE>: <message>` on stderr (`FILE` exactly as given to `-f`, `-` for stdin; `LINE` 1-based physical line; exactly one error is reported, the lowest line number first, except the cycle rule above), and nothing on stdout.

### 3.1 Command-line words

The text after `run ` is split into **words** by blanks (spaces/tabs) outside quotes. Within a word, quoting works as in a POSIX shell restricted to: `"..."` (backslash escapes only `\"` and `\\`; other backslashes are literal; `${...}` and `$$` are expanded inside), `'...'` (everything literal, no expansion, no escapes), and adjacency (`a"b c"'d'` is the single word `ab cd`). `""` is a legitimate empty argument. An unterminated quote is a load error. There is no globbing, no tilde expansion, no command substitution, no redirection, no pipes, no `;`: all those characters are ordinary text. Backslash outside quotes is ordinary text.

### 3.2 Variables

`${NAME}` inserts the value of variable `NAME` (from `set`, overridden by `-D`); `${env:NAME}` inserts the process environment value of `NAME` (an unset variable yields the empty string); `$$` is a literal `$`. Any other `$` (e.g. `$x`, `${`, `${a b}`, trailing `$`) in a `set` value or a `run` line (outside single quotes) is a load error. Variable values may themselves contain `${...}` references (expanded recursively, at use). Expansion happens on the words **after** word splitting, so an inserted value is always part of exactly one argument and its blanks, quotes, `$`, `;`, `*`, etc. have no further effect; inserted text is never expanded again (only variable VALUES are expanded, once, at insertion), and a word that expands to the empty string remains an empty argument (words are never dropped). A reference to an undefined variable and a cycle among variables (`A -> B -> A`) are errors found by `run` only (exit 65, `taskflow: <FILE>:<LINE>: undefined variable NAME` / `... variable cycle: A -> B -> A`). `run` expands the `run` lines of the planned tasks in plan order, each line left to right, and reports the first problem met. For an undefined variable LINE is the line containing the reference (the `set` line when the reference is inside a `set` value being expanded, otherwise the `run` line); for a cycle LINE is the `set` line of the cycle member that appears first in the file. Variables that are never expanded by the planned tasks cannot cause an error; `list` and `plan` never expand variables.

## 4. Functional requirements

**R1 (CLI shell).** Section 2: dispatch, option parsing, usage errors, `help`, `version`, default and unknown task handling, exit codes, `-f -`.

**R2 (Flowfile parsing).** Section 3 grammar and all load errors with correct `FILE:LINE`, the 1000-task limit, CRLF handling, comments/blank lines, indentation by space or tab, zero-`run` tasks, `desc`.

**R3 (variables).** Section 3.2: `set`, `-D` override, `${env:}`, `$$`, recursive values, undefined variable, cycles, invalid `$` forms, no re-splitting of values.

**R4 (command words and argv fidelity).** Section 3.1. The argv received by the child process equals the expanded words exactly (byte-exact, including UTF-8, empty arguments, embedded quotes/blanks/newlines from `-D` values). The program (first word after expansion) MUST be an absolute path; otherwise `run` exits 65 `taskflow: <FILE>:<LINE>: program must be an absolute path` before executing anything. No shell is ever involved: metacharacters in arguments have no effect (acceptance injects `;`, `|`, `$(...)`, backticks, `*`, `>`).

**R5 (graph, `list`, `plan`).** Plan = depth-first post-order: for each requested task in the given order, visit its dependencies in declaration order (recursively), then the task; a task already in the plan is not repeated. `plan` prints one line per planned task: `<N><TAB><name>` with N from 1. `list` prints one line per task in the file, sorted by name (byte order): `<name><TAB><deps comma-separated or -><TAB><desc or ->`. Both load the file with the full parse validation of R2 (but never expand variables or touch the environment).

**R6 (execution and output).** `run` first loads, plans and **expands every `run` line of every planned task** (so that program-path errors, undefined variables and environment denial surface before anything runs), then executes the plan in order. stdout format, exactly:

```
==> NAME
| <line of combined output>       (one per output line)
<== NAME ok
```

A task's commands run sequentially; the child's combined stdout+stderr (as provided by the runtime) is printed, each `\n`-terminated line prefixed by `| ` (an empty line becomes `| `; a final unterminated line is still printed and terminated; no output => no `| ` lines). A task with zero `run` lines prints only its `==>`/`<==` lines. After all tasks: `SUMMARY ok=A failed=B skipped=C`. A task is `failed` when a command exits non-zero (`<== NAME FAILED (exit N)`), cannot be started or fails abnormally (`FAILED (error)`), exceeds the runner's exec-output limit (`FAILED (output limit exceeded)`) or is denied by the runtime (`FAILED (denied)`); the remaining commands of that task are not run. The output a failing command produced before failing is shown (R7 note). A skipped task prints only `<== NAME skipped (REASON)` (no `==>` line).

**R7 (failure policy).** Default is fail-fast: after the first failed task nothing further is started; every planned task not yet started is skipped with reason `aborted`. With `--keep-going`: every planned task whose dependencies all succeeded still runs; a task with a failed or skipped dependency is skipped with reason `dependency D failed` where D is its first dependency (declaration order) whose status is not ok. Exit status: the rule of section 2 (77 > 1 > 0). Note: the runtime reports a non-zero exit only as an error; the implementation must still recover the child's exit code and its output (a runtime enhancement is permitted if needed; the requirement is not).

**R8 (`--dry-run`).** Plans and expands exactly as `run`, executes nothing, writes no report/history, reads NO environment (`${env:NAME}` stays verbatim as text in the displayed word) and needs no process grant. Output: `==> NAME`, then per command `| would run: <argv>` where each word is shown bare if it matches `[A-Za-z0-9_@%+=:,./-]+`, else wrapped in single quotes with every embedded `'` written as `'\''` (an empty word is `''`), words joined by one space; then `<== NAME dry-run`; finally `SUMMARY dry-run tasks=N`. Exit 0.

**R9 (`--report FILE`).** After a (non-dry) run, even a failed one, FILE contains JSON `{"version":1,"status":"ok"|"failed","targets":[...],"tasks":[{"name","status","exit","reason","commands"}...]}`: `targets` = the requested task names (`["default"]` if none given); `tasks` in plan order; `status` per task `ok`/`failed`/`skipped`; `exit` = child exit code (0 for ok tasks, integer for `FAILED (exit N)`, else null); `reason` = null for ok, otherwise one of `exit`, `error`, `output_limit`, `denied`, `aborted`, `dependency_failed`; `commands` = number of commands started. Top-level `status` is `ok` iff every task is ok. Before the first command runs the program proves it can write FILE by writing a provisional document there (`{"version":1,"status":"running",...}`); if that is denied it exits 77 and nothing was executed.

**R10 (`--history FILE` and `history`).** FILE holds `{"version":1,"runs":[{"started":<unix s>,"targets":[...],"status":"ok"|"failed","ok":A,"failed":B,"skipped":C}...]}`, oldest first, at most the LAST 50 runs. A non-dry `run` with `--history` appends one record after execution (creating the file if absent). The file is read and validated first, and its writability proven (by rewriting it unchanged, or creating an empty history) before the first command runs: invalid history => exit 65, denied => exit 77, in both cases nothing executed. `taskflow history FILE` prints one line per record (oldest first): `<started><TAB><status><TAB><targets comma-separated><TAB><ok><TAB><failed><TAB><skipped>`; `--last N` prints only the last N. Missing FILE => 66; invalid => 65.

**R11 (process capability).** Every command is executed only through the runtime's governed exec. Required grants: `process:exec=<absolute canonical program path>[ <argv patterns>]` per program (see `docs/reference/process_authority.md`). A command the runtime denies makes its task `FAILED (denied)`, is reported on stderr as exactly one line `taskflow: capability denied: process (<task name>)`, and is never retried or run another way; the program and arguments are never echoed in the denial line. Children get an environment holding only `process:env=` grants. The program MUST NOT require `process` (the coarse grant), `network` or `database`.

**R12 (environment capability).** `${env:NAME}` requires the `environment` grant and is evaluated only in `run` (never `list`, `plan`, `--dry-run`). If denied: exit 77 during planning, one stderr line `taskflow: capability denied: environment`, nothing executed, nothing written (no report/history). A Flowfile without `${env:` needs no `environment` grant.

**R13 (filesystem capability).** Reading the Flowfile (`-f FILE`/default, except `-f -`), reading `--history`/`history` FILE needs `filesystem:read=<dir>`; writing `--report`/`--history` needs `filesystem:write=<dir>` (read is not implied). Denied => exit 77, `taskflow: capability denied: filesystem`, no command run, no file created or modified. A path outside every granted root is a denial; a missing Flowfile inside a granted root is exit 66.

**R14 (resource limits).** (a) more than 1000 tasks => 70. (b) Runner `--max-exec-output-bytes N`: a command producing more than N bytes of output fails its task with `FAILED (output limit exceeded)` (no part of the oversize output is printed); the run continues according to R7. (c) Runner `--deadline D`: the run stops promptly (non-zero exit, no hang) even while a child is running. (d) Runner `--max-instructions`/`--max-memory-bytes` exhaustion stops the program non-zero without printing success text; no hang.

**R15 (determinism and hygiene).** Same inputs => byte-identical stdout (timestamps appear only in history/report files). Every stderr line starts with `taskflow: `; stderr is empty on successful runs; no raw runtime error objects, stack traces, environment values or secret values are printed anywhere (stdout, stderr, report, history); a plan of 300 chained tasks is computed correctly and quickly.

## 5. Edge cases (binding)

Duplicate `set`/`task`; unknown directive at column 0 and indented; indented line before first task; `task` with `after` but no deps, or trailing words; invalid names; tab-indented bodies; CRLF Flowfile; comments inside task bodies; empty `run`; unterminated `"`/`'`; `\"` and `\\` inside double quotes; `$$`, `$x`, `${`, `${a b}`, `${env:}`, `${}`; value with spaces/quotes/`;`/`*`/newline from `-D`; empty `-D` value; `-D` overriding `set`; variable defined only by `-D`; relative program; a program that does not exist; a command that exits 0/3/255; output without trailing newline; output with empty lines; very chatty output (limit); diamond dependencies (task planned once); duplicate targets on the command line; self-dependency (`task a after a`); two-node and three-node cycles; unknown dependency; unreferenced cyclic tasks; `default` absent; `--keep-going` with independent branches; a denied command in a middle task; report/history on a failed run; history trimmed to 50; corrupt history.

## 6. Out of scope

Parallel execution, file-timestamp/content based up-to-date checks, caching, pattern rules, globbing, shell syntax of any kind, per-task environment settings, working-directory changes, includes, conditionals/loops in the Flowfile, remote execution, watch mode, colored output, interactive prompts, signal handling beyond the runner's deadline.
