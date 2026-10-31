# WebAssembly GC backend

Status: milestones M1 and M2 done (2026-10-30); M3 in progress.

## Goal

Compile HowlFrame programs directly to WebAssembly that runs in browsers,
Node.js and standalone engines, so that a complete application (the first
one is HowlScene HF: editor, engine and published-site runtime) is written
in HowlFrame and executed as HowlFrame-compiled code, with no JavaScript,
Go or other general-purpose language in the application.

This backend is a new target beside the bytecode VM, Go and JavaScript
backends. It does not replace the VM, which stays HowlFrame's verified
execution engine for servers and command-line programs.

## Decisions

| Decision | Choice | Why |
| --- | --- | --- |
| Reference semantics | The bytecode VM | It is the verified engine. Where the JavaScript backend differs (for example `str_len`: the VM counts characters, the JavaScript backend UTF-16 units), this backend follows the VM. Every construct is checked against the VM by the differential harness (`tools/difftest`, new target `wasm`). |
| Memory | WasmGC structs and arrays | The engine's garbage collector manages strings, lists, dicts and closures; no allocator or collector of our own. Supported in Chrome 119, Firefox 120, Safari 18.2, Node 22 and wasmtime. |
| Output | Binary `.wasm` written by the compiler | No external assembler is needed to build. A text (WAT) dump is available for debugging. `wasm-tools`, Node and wasmtime are used by tests as independent validators and engines. |
| Errors | Explicit propagation, no exception handling | A failing operation sets an error value and returns; callers check and pass it up to the nearest `try_let`, `catch` or the program's top level. This avoids depending on either version of WebAssembly exception handling, which engines support unevenly. |
| Core runtime | Emitted by the compiler | The operations every value needs (allocation, equality, dict lookup and insertion, list growth, string concatenation and comparison, printing) are WebAssembly functions the compiler writes with its instruction builder, as the JavaScript backend emits its helpers. They are part of the platform, not of any program. |
| Standard library | Written in HowlFrame | JSON, regular expressions, HTML escaping, base64, UTF-8 checks and number formatting are HowlFrame modules embedded in the compiler (`std/`), compiled into a program when it reaches them. They are built on a small set of byte-level intrinsics (byte length, byte at, build bytes) that only the standard library may call. |
| Host access | Typed imports, capability-gated | Printing, arguments and exit for command-line programs, and for browser programs the DOM, events, storage, files and images, are imports from a host module the compiler generates from a typed table. Each import names the capability it needs; a program that is not granted it cannot link. The host module is the only non-HowlFrame code, and it is generated, generic and the same for every program. |

## Value representation

Every HowlFrame value is a nullable `eqref`. Type tests use `ref.test` and
casts `ref.cast`.

| HowlFrame | WasmGC |
| --- | --- |
| `nil` | `ref.null none` |
| `true`, `false` | `$Bool` struct `(field i32)`; two immutable globals |
| integer | `$Int` struct `(field i64)`; 64-bit two's complement like the VM |
| float | `$Float` struct `(field f64)` |
| string | `$Str` struct `(field (ref $Bytes))`, `$Bytes` = `(array i8)`, UTF-8 |
| list | `$List` struct `(field (mut i32) len) (field (mut (ref $Items)) items)`, `$Items` = `(array (mut eqref))`, growing by doubling |
| dict | `$Dict` struct `(field (mut i32) len) (field (mut (ref $Items)) keys) (field (mut (ref $Items)) values)`: insertion-ordered entries, keys as `$Str`, found by a linear scan for now (an index can be added later without changing behavior). A dict is a reference: `map_set` and `map_delete` change it in place. |
| closure (`lambda`) | `$Closure` struct `(field (ref $Fn)) (field (ref $Items))` (code and captured environment) |
| host object (DOM element, event, subscription) | `$Host` struct `(field externref)` |

Integers stay boxed in M1; type hints let a later optimization keep
`int`/`float` locals unboxed.

Printing follows the VM (`fmt.Sprint`): integers in decimal, floats in
Go's shortest form, `true`/`false`, nil as the empty string (but `<nil>` inside a
list or dict), lists as `[a b]`, dicts as `map[k:v ...]` with sorted keys.

## Functions and calls

Each `defun` becomes a WebAssembly function taking and returning `eqref`;
arity is checked at compile time. Modules (`use`) are linked before any
backend runs (`ast.ResolveModules`): a used module's exports become
`alias_name` and its other definitions `alias_private_name`, so they compile
into the same WebAssembly module and cross-module calls are direct calls. Lambdas are lifted to functions with an environment array;
captured variables that are assigned (`set`) are boxed in a one-slot cell.

## Error propagation

Each function returns its value and leaves a global `$err` null on success.
After any call or operation that can fail, the caller tests `$err`; if set,
it returns at once (or jumps to its `try_let` handler, which clears `$err`
and binds the error). `$err` holds `CODE: message`, with the VM's error code
(`TYPE_ERROR`, `RUNTIME_ERROR`, `CONVERSION_ERROR`) and message. The top
level reports an uncaught error on standard error as the VM's JSON report
and exits 1. A `try_let` binds the VM's report too:
`{"phase":"runtime","code":...,"function":"main","message":...}`, with the
message escaped as Go's `encoding/json` does. The VM's report also has
`instruction` (and usually `opcode`) fields naming the VM's own bytecode
position; they have no WebAssembly meaning and are left out, so a program
that prints a caught error whole prints different text on the two engines.

## Asynchronous host operations

Browser storage, file reading and image decoding complete later. HowlFrame
source calls them like ordinary functions (the JavaScript backend awaits
them). Without relying on JavaScript Promise Integration, which is not
available in every engine, the compiler turns a function that reaches an
asynchronous operation into a resumable state machine: its locals live in a
frame struct, the asynchronous import receives a continuation, and the host
resumes the frame with the result. This is decided in M3, with measurements;
JSPI remains an option where available.

## Milestones and exit gates

| Milestone | Exit gate |
| --- | --- |
| M1 core language | `build --target wasm` (new backend) compiles `cli_app` programs using values, control flow, functions, modules, strings, lists and dicts; a corpus runs identically on the VM and on WebAssembly (Node and wasmtime); HowlScene HF's schema, registry and renderer compile and produce byte-identical HTML to the VM. |
| M2 tests | `test` blocks compile to WebAssembly and run under a generated runner; HowlScene HF's core tests pass on WebAssembly. |
| M3 browser host | DOM, events and closures, storage, files, images and ZIP as typed, capability-gated imports; asynchronous operations resolved; HowlFrame's browser tests pass on WebAssembly. |
| M4 site runtime | HowlScene HF's published-site runtime runs on WebAssembly; all motion and story tests pass; initial download within the 250 KiB budget. |
| M5 editor | The full HowlScene HF browser suite passes on WebAssembly, including the beginner journey; HowlScene HF stops using the JavaScript backend. |
| M6 no other languages in the app | Styles and the editor's page shell are generated from `.howl`; the source policy allows only `.howl` under `src/`. |

## Testing

- Unit tests of the binary encoder: every module validates with
  `new WebAssembly.Module` in Node and with `wasm-tools validate`.
- Differential tests: every corpus program on the VM and on WebAssembly,
  outputs and exit codes equal (`tools/difftest` target `wasm`).
- Each new construct starts with a failing differential fixture.
- HowlScene HF's own suites are the application-level gate for M2 to M5.

## Working plan

This section is the plan to work from. Update it in the same pull request
as the work it describes: tick tasks, move the status line, record findings.

### Status

| Milestone | State | Notes |
| --- | --- | --- |
| M1 core language | Done (2026-10-30) | `build --target wasmgc` compiles `cli_app` programs using values, control flow, functions, modules, strings, lists and dicts, with `try_let`, the core-library sugar and a standard library written in HowlFrame (JSON, regular expressions, UTF-8, floats). 365 differential fixtures and the corpus (`TestWasmGCCorpusParity`, 17 programs) match the VM in Node and, built with `--host wasi`, in wasmtime; HowlScene HF's schema and renderer produce byte-identical output on both (`TestHowlSceneGate`). Unicode case mapping and trim, `bytes_to_string`, `utf8_valid`, hexadecimal floats and base64 done; all five HowlScene HF core modules compile. |
| M2 tests | Done (2026-10-30) | `build --target wasmgc --tests` compiles a file's test blocks (any root) into `app.test.wasm` with a runner that reports like node:test, in Node and in wasmtime. 13 report fixtures match the JavaScript backend's `app.test.js` (`TestTestsLikeJavaScript`) and 5 build errors fail closed (`TestTestsFailClosed`). HowlScene HF's 19 core tests (backup 6, render 5, schema 8) pass on WebAssembly with the same report as `scripts/test_core.sh` (`TestHowlSceneTestsGate`); ZIP is in `std/zip.howl` for the backup tests. |
| M3 browser host | In progress (2026-10-31) | Probe done: "Browser host operations" lists every operation with its parameters, result, errors, capability and whether it is asynchronous. Journal: `docs/journals/2026-10-31_wasmgc_m3_journal.md`. |
| M4 site runtime | Not started | |
| M5 editor | Not started | |
| M6 no other languages in the app | Not started | |

### Rules

1. **Test first.** Each construct starts as a failing fixture that runs on the
   VM and on WebAssembly with equal output. A fixture the VM rejects is not
   used.
2. **The VM is the reference.** A difference from the VM is a bug in this
   backend unless the VM is shown to be wrong, in which case the VM is fixed
   first, in its own pull request, with its own test.
3. **No silent fallbacks.** An unsupported construct fails the build with
   `WASM_UNSUPPORTED` naming the construct and its source location, like
   `HFIR_TARGET_INFEASIBLE` on the bytecode target.
4. **Small pull requests.** One coherent slice per pull request (for example
   "strings and string operations"), each passing `go test ./...` and the
   differential corpus. Record each finding about other targets in
   `bugs.md` with a test.
5. **Validators are independent.** Every emitted module is checked with
   `new WebAssembly.Module` in Node and, when installed, `wasm-tools validate`.
   Programs also run under wasmtime where no browser host is needed.
6. **Nothing application-specific.** Every operation added is general; the
   HowlScene HF needs below only set the order of work.

### M1 tasks: core language

- [x] Binary encoder: types in one recursion group, imports, functions,
      globals, exports, declarative element segment, passive data, code.
- [x] Instruction builder: control flow, locals and globals, i32, i64 and
      f64 arithmetic, conversions, references, GC struct and array
      instructions, casts.
- [x] Runtime value types (`$Bool`, `$Int`, `$Float`, `$Str`, `$Bytes`,
      `$List`, `$Items`, `$Dict`) and the error global. (`$Closure`,
      `$Host` come with M3.)
- [x] Core runtime functions: boxing, truthiness, equality, comparison,
      arithmetic with the VM's integer and float rules and division errors,
      string concatenation and comparison, printing of scalars as
      `fmt.Sprint` (float digits from `std/float.howl`), `to_int`,
      `to_float`, `to_string`.
- [x] Printing of lists and dicts (`[a b]`, `map[k:v]` with sorted keys,
      `<nil>` for a nil element).
- [x] `build --target wasmgc` (named `wasm` once it covers the legacy WAT
      target): writes `app.wasm` and a generated loader (`app.mjs`) that
      runs a `cli_app` in Node with print as a host import. Arguments and
      exit to follow.
- [x] `tools/difftest` target `wasmgc`, run in Node: `TestWasmGCCorpusParity`
      builds every `cli_app` of the corpus (`tests/*.howl` less the
      manifest's exemptions, and `examples/*.howl`) and compares it with
      the VM. 17 match; 4 stop at host operations (`cli_args`, `env`,
      `read_line`) or an adaptive construct (`optimize_signature`), counted
      as not supported yet. Typed parameters (`((a int) (b int))`) were the
      one gap it found.
- [x] The same corpus under wasmtime: `build --target wasmgc --host wasi`
      writes a WASI command (`_start`, imports `fd_write` and
      `proc_exit`, a one-page memory for output); every differential
      fixture, the corpus and the HowlScene gate also run in wasmtime when
      it is installed. Its import signatures are standalone types: inside
      the module's recursion group wasmtime refused them as a different
      type.
- [x] Literals and `print`; `let`, `set`, `do`, `if`, `while`;
      arithmetic and comparison; `and`, `or`; `defun`, `call`, `return` and
      recursion; `to_int`, `to_float`, `to_string`.
- [x] Lists: `list`, `append`, `list_get`, `list_len`, `for`; list
      printing; `list_slice` and `list_contains` (desugared). Lists are
      values: an immutable (length, buffer) pair where only the list at the
      buffer's tip appends in place.
- [x] String primitives `str_split`, `str_join`, `to_lower`, `to_upper`,
      `str_trim`, `html_escape`, `attr_escape`, `is_nil`; through their
      desugaring `str_len`, `str_slice`, `str_contains`, `str_replace`,
      `starts_with`, `ends_with`; `%` and `//`; `stderr` and `exit`.
- [x] Dicts: `dict`, `map_get`, `map_set`, `map_keys`, `map_delete`, and
      `map_has` through its desugaring; type errors name the value's Go type
      (`%T`) as the VM's do, also for `for`, `append`, `list_get` and
      `list_len` (which gives a string's byte length).
- [x] `not` and the sugar, after desugaring, each with a fixture
      (`TestDiffSugar`): `cond`, `switch`, n-ary `if`, `and` and `or`,
      `when`, `unless`, `inc`, `dec`, `str`, `nz`, `blank`, `all`, `any`,
      `ver_gt`, `ver_ge`, `index_of`, `map_list`, `filter`, `reduce`, `sum`,
      `min`, `max`, `reverse`, `count_by`, `sort`, `range`, `sort_by`,
      `max_by`, `min_by`, `pad_left`, `pad_right`, `str_repeat`, `ltrim`,
      `rtrim`, `to_base`, and the records forms `rec`, `get`, `put`, `pluck`,
      `select`, `group_by`, `merge`. None needed new runtime code.
- [x] `try_let` and `catch`, as statements and values, nested, in loops
      and across calls; `exit` and failures in either body are not caught.
      Errors carry the VM's codes, and the VM's run-time rules hold:
      conditions (`if`, `while`, `and`, `or`, `not`) must be booleans,
      float `+`, `-`, `*` and `/` fail when the result is not finite, and
      operands of unknown type fail with the VM's messages.
- [x] Modules (`use`, `export`): linked before the backend, so no backend
      code was needed; `TestDiffModules` runs programs of several files
      (exports, private helpers that share a name with the program's own
      functions, two modules, a subdirectory, errors raised and caught
      across modules).
- [x] Standard-library pipeline: `internal/backend/wasmgc/std/*.howl` are
      embedded modules, parsed, linked as `__std_<module>` and desugared
      when a program reaches one of their operations (`stdOps`). Only their
      functions may use the byte intrinsics `__hf_type` (the VM's `%T`
      name), `__hf_byte_len`, `__hf_byte_at`, `__hf_byte_slice`, integer
      operations (`__hf_idiv`, `__hf_imod`, `__hf_shl`, `__hf_shr`,
      `__hf_and`, `__hf_or`) and a float's bits (`__hf_f64_bits`,
      `__hf_f64_from_bits`), a mutable array
      (`__hf_array`, `__hf_aget`, `__hf_aset`, `__hf_alen`) for std
      algorithms, and `__hf_fail` to raise an error with a code; a program
      that calls one, or defines a `__std_` function, fails with
      `WASM_UNSUPPORTED` (`TestStdIsReserved`). A std module may call
      another by its linked name (`__std_utf8_seq_len`), which loads it.
- [x] `encode_json` in `std/json.howl`, as Go's `encoding/json` writes the
      VM's values (`TestDiffEncodeJSON`).
- [x] Floats in `std/float.howl`: Go's arbitrary-precision decimal
      (`strconv/decimal.go`) ported to HowlFrame. `roundShortest` gives the
      shortest round-trip digits for printing, and `floatBits` the nearest
      float for `to_float`, in `ParseFloat`'s decimal syntax with its
      underscore rule. The runtime calls it through two bridges, and a
      program that cannot make a float leaves the module out (19 KB).
      `TestDiffFloats` compares 729 values: powers of ten, subnormal
      halvings, ratio sequences, 300 random texts from a fixed seed and
      hard cases (halfway, boundaries, 800+ digits).
- [x] `parse_json` in `std/json.howl`, as the VM's (Go's `json.Decoder`
      with `UseNumber`, then a check for a second value): ints when they fit
      int64, otherwise floats; the last duplicate key wins; U+FFFD for
      invalid UTF-8 and unpaired surrogates; Go's syntax messages with its
      quoting of the offending byte, `unexpected EOF`, `EOF`, the
      second-value messages, and float range errors reported after them
      (`TestDiffParseJSON`). A byte list as the text fails with
      `WASM_UNSUPPORTED`. The messages are Go 1.25's, which the VM now
      gives whatever the toolchain (bugs.md #63, fixed).
- [x] `regex_match` in `std/regex.howl`: Go's `regexp.MatchString` for a
      documented RE2 subset (literals, `.`, classes with ranges and Perl
      escapes, `^`, `$`, groups including named and non-capturing ones,
      `|`, `* + ? {n} {n,} {n,m}` and their lazy forms), by Thompson NFA
      simulation over code points, linear in the text. Invalid patterns
      fail with the VM's `INVALID_REGEX` messages; syntax outside the subset
      (`(?i)`, `\b`, `\p{..}`, `\x`, octal, `\Q..\E`, `[[:alpha:]]`,
      `\A`, `\z`) fails with `WASM_UNSUPPORTED` at run time
      (`TestDiffRegex`, `TestRegexUnsupportedFailsClosed`).
- [x] Unicode case mapping and trim: `to_lower` and `to_upper` as Go's
      `strings.ToLower` and `ToUpper` through `std/unicode.howl`, generated
      from `internal/textcase`'s table, which the VM uses too (Unicode
      15.0.0 whatever the toolchain, bugs.md #69;
      `TestUnicodeTableIsCurrent` keeps it in step on every toolchain), with the
      ASCII fast path in the runtime; `str_trim`, `to_int` and `to_float`
      trim Go's `unicode.IsSpace` set. Nil text is `<nil>` (`<NIL>` upper
      case), as `fmt.Sprint` gives it; this backend gave "" before.
      `TestDiffUnicodeText` sweeps eight long-assigned blocks, and its
      `unicode 17 cased` fixture the code points Unicode 17 cased, which
      both engines leave alone.
- [x] `bytes_to_string` (a list of byte values to a string, each as Go's
      `byte(float64(n))` on amd64) and `utf8_valid` (`std/utf8.howl`,
      Go's `utf8.ValidString`). `bytes_to_string` is how a program makes
      text that is not valid UTF-8, so `TestDiffBytes` also runs such text
      through printing, JSON, splitting, case mapping, regular expressions
      and escaping. The VM's `[]byte` values (from files and the network)
      arrive with M3's host operations.
- [x] base64: `base64_text` and `sniff_media_type` in `std/base64.howl`.
      They are JavaScript-backend host operations with no VM form, so the
      JavaScript backend is their reference (`TestBase64LikeJavaScript`
      runs each program as a `web_app` there): its validation (length a
      multiple of four, the standard alphabet, at most two `=`), `atob`'s
      decoding, `TextDecoder`'s strict UTF-8 with a leading BOM dropped,
      and the four image signatures. With them HowlScene HF's backup
      module compiles, and `TestHowlSceneBackupGate` runs it (writing a
      backup, reading good and broken ones) on both WebAssembly engines
      against the JavaScript backend.
- [x] Hexadecimal floats in `to_float`: Go's `readFloat` for base 16 and
      `atofHex` in `std/float.howl` (16 mantissa digits with a sticky bit,
      the binary exponent required, rounding half to even, subnormals and
      overflow). `TestDiffHexFloats` compares 236 values, 32 hard cases
      and 200 texts from a fixed seed.
- [x] Exit gate, rendering: HowlScene HF's registry, schema and renderer
      compile; rendering its fixture projects gives byte-identical output
      on the VM and on WebAssembly (21,872 bytes for the two neutral
      fixtures, rendered two ways and re-encoded). `TestHowlSceneGate` runs
      it when `HOWLSCENE_HF` names a checkout; evidence in
      `docs/journals/2026-10-29_howlscene_hf_wasmgc_m1_gate.md`.
- [x] Exit gate, engines: the corpus runs identically on the VM, in Node
      and in wasmtime (2026-10-30).

### M2 tasks: tests

- [x] Harness hygiene: the differential harness runs every command in a
      temporary directory, so the VM's bare `howlframe prog.howl` no longer
      writes `prog.hfbc` into the package; `TestMain` fails a run that
      leaves one there.
- [x] Reference probe (2026-10-30): `test` and `assert` on the JavaScript
      backend (the reference: `app.test.js` under `node --test`) and on the
      Go backend (`server_test.go` under `go test`). See "Test blocks"
      below and the findings.
- [x] Roots: a test mode, `build --target wasmgc --tests`, compiles a
      file's test blocks whatever its root (`cli_app` or `web_app`); see
      "Test blocks" for the choice.
- [x] `test` blocks and `assert` compile into a test module
      (`app.test.wasm`) with one export per test (`test_0`, `test_1`,
      ...) and a `main` that runs them and reports like `node:test`'s spec
      reporter; `app.test.mjs` runs it in Node, and with `--host wasi` it is
      a WASI command for wasmtime. `TestTestsLikeJavaScript` compares every
      fixture's report with the JavaScript backend's (results in order,
      each failing test's error, counts, output, exit status) in Node and
      wasmtime: passing and failing tests, errors in a test, an assert in a
      called function, `exit`, `exit` through `try_let`, an empty file,
      top-level statements, a `cli_app` root, tests in used modules, equal
      names, a Unicode name and type errors on values of run-time type. `TestTestsFailClosed` covers the build
      errors.
- [x] HowlScene HF's `tests/core/*_test.howl` pass on WebAssembly: backup
      (6), render (5) and schema (8), with ZIP in `std/zip.howl` (#277).
- [x] Exit gate: `TestHowlSceneTestsGate` builds each core test file with
      `--tests` and runs it in Node and wasmtime; each report (results in
      order, errors, counts, exit status) must equal the JavaScript
      backend's `app.test.js` under `node --test`, which is what
      `scripts/test_core.sh` runs, and that run must pass. Evidence
      (2026-10-30, HowlScene HF `247365e`): 19 of 19 pass in both engines;
      `scripts/test_core.sh` with its pinned compiler (`8e16f799`) also
      passes 19 of 19. Runs with `HOWLSCENE_HF` set (and in
      `scripts/wasmgc_gate.sh`).

### Test blocks

A test file's tests run like the JavaScript backend's `app.test.js`, which
is the reference (the VM has no test runner; improvements.md #96):

- The test module holds the program's functions and its test blocks, in
  source order; a used module's tests come where its `use` is (modules are
  linked in place). Top-level statements are the application's and are
  not run, as `app.test.js` leaves them out.
- A test passes when it returns with no error. An uncaught error fails it
  with `CODE: message`; `exit` (and so a failing `assert`, which writes
  `assertion failed` to standard error and exits 1) fails it with
  `HowlFrame exit N at file:line`, the JavaScript backend's text. No
  `try_let` catches an exit (a `$exiting` flag makes every handler pass the
  error on), as the VM's exit ends the program whatever encloses it. The
  next test then runs.
- The report is the spec reporter's without durations: `✔ name` or
  `✖ name` with `  Error: ...` under it, then `ℹ tests`, `suites`,
  `pass`, `fail`, `cancelled`, `skipped`, `todo`. Exit status 1 when a
  test failed. Prints and standard error go where they go in a program.
- The runner is part of the module (its `main`), so Node and wasmtime run
  the same code and report the same text; the Node runner is the ordinary
  generated loader pointed at `app.test.wasm`.

Roots. HowlScene HF's test files are `web_app` roots, and a `web_app`
program needs the browser host (M3). Two ways were weighed:

| Option | For | Against |
| --- | --- | --- |
| Accept a `web_app` that uses no browser host operation | No new flag; the same command builds programs and tests | A `web_app`'s top level is browser bootstrap code: running it as a command-line program gives it a meaning it does not have, and a test file whose top level touches the DOM could not have its tests built. Settles M3's `web_app` design early. |
| A test mode that compiles a file's test blocks whatever its root (chosen) | Matches `app.test.js` exactly (functions and tests, no top level); independent of the browser host; explicit about what is built | One more flag (`--tests`); a program and its tests are two builds |

`--tests` writes `app.test.wasm` (and `app.test.mjs` for Node) instead of
the program. Without it, a `web_app` still fails with
`WASM_UNSUPPORTED` (naming `--tests`), and a `cli_app` with test blocks
fails on its first `test`, as the VM's bytecode target refuses them.

### M3 tasks: browser host

- [x] Probe (2026-10-31): every browser host operation of the JavaScript
      backend and HowlScene HF's use of each; see "Browser host operations".
- [x] Typed host operation table (name, parameter and result kinds,
      capability, synchronous or asynchronous) shared by the checker and the
      backends: `capability.HostOperations` (`internal/capability/host.go`).
      The checker takes arity, usage and lambda arguments from it (its two
      maps and eleven inline branches are gone); the desugarer's set of
      browser operations is held equal to it
      (`TestEveryJSHostOperationExpandsNestedSugar`), the JavaScript
      backend's awaits to its asynchronous flags
      (`TestHostOperationsMatchRegistry`), and every operation's arity is
      checked from it (`TestEveryBrowserHostOperationArity`). It lives in
      `internal/capability`, which every consumer may import (`ast` may not
      import it without a cycle through `construct`, hence the held-equal
      set). The browser loader is generated from it with the browser host.
- [x] Host object handles (`$Host` with `externref`) and a `web_app` root
      that builds for the browser (`--host browser`, the default for a
      `web_app`): `app.wasm` and a generated loader, `app.js`. See "Browser
      host" below. `TestBrowserLikeJavaScript` runs 18 programs (every
      synchronous DOM, layout, preference and style operation, values of
      every kind, missing elements, caught and uncaught errors, `exit`,
      `stderr`, refused downloads, test blocks left out) on both backends
      against a stand-in page and requires the same page.
- [x] Closures and event dispatch: `lambda`, `on_event`, `spawn`, and the
      subscription operations (`on_media`, `on_visible`, `on_scroll`, `off`).
      See "Handlers" below. `TestBrowserHandlersLikeJavaScript` runs 12
      programs (shared and per-iteration captures, lists and dicts, nested
      handlers, handlers made in a function, handler errors and exits,
      a missing element, `spawn`, the subscriptions and `off`) on both
      backends, clicking, scrolling, flipping a media query and making an
      element visible on the stand-in page.
- [ ] Asynchronous operations (storage, files, image decode and encode, ZIP,
      `sleep`): resumable frames, decided with measurements; JSPI where
      available is the alternative.
- [ ] Every DOM, storage, file, image, archive and rich text operation the
      JavaScript backend supports, with the same errors.
- [x] ZIP, done early for M2's exit gate (HowlScene HF's backup tests read
      and write ZIPs): `zip_build` and `zip_read` in `std/zip.howl`, on
      base64 text as on the JavaScript backend, with CRC-32, base64
      encoding and an inflate for deflated entries ported from zlib's
      reference decoder (puff), with zlib's rules for incomplete and
      over-subscribed codes. `TestZipLikeJavaScript` runs 74 programs
      against the JavaScript backend in Node and wasmtime: every
      `zip_build` check and message, archives from Go's `archive/zip`
      (stored, fixed and dynamic Huffman, stored blocks, level 0, empty and
      directory entries) and hand-made damaged ones (every `ZIP_INVALID`,
      `ZIP_INVALID_PATH`, `ZIP_DUPLICATE_PATH`, `ZIP_LIMIT`,
      `ZIP_UNSUPPORTED` and `ZIP_CORRUPT` case, names quoted as
      `JSON.stringify` quotes them, a cut surrogate pair included).
- [ ] HowlFrame's browser tests (`internal/backend/javascript/browser_*`)
      ported to run against both backends.

### Browser host operations (M3 probe, 2026-10-31)

Every browser host operation of the JavaScript backend (the reference), read
from `dom.go`, `browser_io.go`, `motion.go`, `richtext.go`, `archive.go`,
`exit.go` and `javascript.go`, and from the checker's `jsHostOperations` and
`jsSubscriptionOperations`. "Async" means the JavaScript backend awaits it.
"Raw" operations are emitted inline (`el.textContent = v`) with no helper:
on a missing element they throw the engine's own `TypeError`, with no
HowlFrame code. Columns HF site and HF editor count uses in HowlScene HF's
`src/site/site.howl` and `src/editor/editor.howl` (`247365e`); its core
modules use none, and `src/sdk` and the editor's `ui.howl` and
`templates.howl` none either.

| Operation | Parameters | Result | Errors | Cap. | Async | HF site | HF editor |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `dom_query` | selector | element or nil | raw (`querySelector`) | – | no | 5 | 1 |
| `dom_query_all` | selector | list of elements | `TYPE_ERROR` | – | no | 4 | – |
| `set_text` | el, value | – | raw | – | no | – | 4 |
| `set_html` | el, html | – | raw | – | no | – | 6 |
| `set_attr` | el, name, value | – | raw | – | no | 4 | 7 |
| `toggle_class` | el, class | bool | raw | – | no | – | – |
| `dom_value` | el | text | raw | – | no | – | 5 |
| `dom_text` | el | text | `TYPE_ERROR` | – | no | – | 6 |
| `set_value` | el, value (nil is "") | – | `TYPE_ERROR` | – | no | – | 3 |
| `dom_checked` | el | bool | `TYPE_ERROR` | – | no | – | – |
| `set_checked` | el, bool | – | `TYPE_ERROR` | – | no | – | – |
| `dom_focus` | el | – | `TYPE_ERROR` | – | no | – | 3 |
| `dom_attr` | el, name | text ("" when absent) | `TYPE_ERROR`, `INVALID_ATTRIBUTE_NAME` | – | no | 2 | – |
| `remove_attr` | el, name | – | `TYPE_ERROR`, `INVALID_ATTRIBUTE_NAME` | – | no | 2 | 2 |
| `dialog_open`, `dialog_close` | el | – | `TYPE_ERROR` | – | no | – | 6, 8 |
| `on_event` | el, type, `(lambda (e) ...)` | – | raw | – | no | 1 | 4 |
| `event_attr` | event, name | text (closest ancestor with it) | `INVALID_ATTRIBUTE_NAME` | – | no | – | 17 |
| `event_key` | event | text | – | – | no | – | – |
| `event_prevent` | event | – | – | – | no | – | – |
| `set_class` | el, name, on | – | `TYPE_ERROR`, `INVALID_CLASS_NAME` | – | no | 9 | – |
| `set_style_var` | el, `--name`, value | – | `TYPE_ERROR`, `INVALID_STYLE_VAR`, `INVALID_STYLE_VALUE` | – | no | 3 | – |
| `scroll_y`, `viewport_height` | – | number | – | – | no | 1, 1 | – |
| `element_box` | el | dict `top`, `height` | `TYPE_ERROR` | – | no | 3 | – |
| `scroll_into_view` | el | – | `TYPE_ERROR` | – | no | 1 | – |
| `media_matches` | query | bool | `TYPE_ERROR` | – | no | 1 | – |
| `pref_get` | key | text ("" when absent or unavailable) | `PREF_INVALID_KEY` | – | no | 1 | – |
| `pref_put` | key, value | bool (false when unavailable) | `PREF_INVALID_KEY`, `TYPE_ERROR` | – | no | 1 | – |
| `on_media` | query, `(lambda (matches) ...)` | subscription | `TYPE_ERROR` | – | no | 1 | – |
| `on_visible` | elements, `(lambda (el visible) ...)` | subscription | `TYPE_ERROR` | – | no | 2 | – |
| `on_scroll` | `(lambda () ...)` | subscription | – | – | no | 1 | – |
| `off` | subscription | – | `TYPE_ERROR` | – | no | 1 | – |
| `subscription_count` | – | int | – | – | no | 1 | – |
| `storage_get` | namespace, key | text or nil | `STORAGE_INVALID_NAME`, `STORAGE_UNAVAILABLE`, `TYPE_ERROR` | – | yes | – | 3 |
| `storage_put` | namespace, key, value | – | as `storage_get`, `STORAGE_QUOTA` | – | yes | – | 4 |
| `storage_delete` | namespace, key | – | as `storage_get` | – | yes | – | – |
| `storage_keys` | namespace | sorted list of text | as `storage_get` | – | yes | – | – |
| `file_read_input` | el, max_bytes | list of dicts `name`, `type`, `size`, `base64` | `TYPE_ERROR`, `FILE_TOO_LARGE` | – | yes | – | 2 |
| `image_decode` | base64, mime | dict `width`, `height` | `UNSUPPORTED_HOST`, `IMAGE_INVALID` | – | yes | – | 1 |
| `image_encode` | base64, mime, max_side, out_mime, quality | dict `base64`, `width`, `height`, `type` | as `image_decode`, `IMAGE_INVALID_OPTIONS` | – | yes | – | 1 |
| `download_file` | name, mime, base64 | – | `DOWNLOAD_INVALID_NAME`, `_TYPE`, `_DATA`, `UNSUPPORTED_HOST` | – | no | – | 2 |
| `richtext_attach` | el, label | – | `TYPE_ERROR` | – | no | – | 1 |
| `richtext_set` | el, blocks | – | `TYPE_ERROR`, `RICHTEXT_INVALID` | – | no | – | 1 |
| `richtext_get` | el | blocks | `TYPE_ERROR` | – | no | – | 2 |
| `richtext_command` | el, command, value | – | `TYPE_ERROR`, `RICHTEXT_UNKNOWN_COMMAND`, `RICHTEXT_INVALID` | – | no | – | 2 |
| `zip_build`, `zip_read`, `base64_text`, `sniff_media_type` | (see ZIP, base64) | | | – | `zip_read` only | – | 2, 1, 1, 1 |
| `url_encode` | text | text (`encodeURIComponent`) | `TYPE_ERROR` | – | no | – | – |
| `fetch` | url, method[, body[, headers]] | text | `CAPABILITY_DENIED` and network errors | network (host-scoped `network:fetch=`) | yes | – | – |
| `sleep` | ms | – | – | – | yes | – | 1 |
| `spawn` | `(lambda () ...)` | – (runs detached) | – | – | starts a task | – | 1 |
| `print`, `stderr`, `exit` | value | – | – | – | no | – | – |

Notes from the probe:

- A lambda appears only as the last argument of `on_event`, `on_media`,
  `on_visible`, `on_scroll` and `spawn` (the checker requires it). It is
  never a value a program can call, store or compare, so closures need an
  invoke path from the host only. JavaScript closures share the enclosing
  variables: a handler's `set` is seen by the code around it and by later
  events. The VM's `spawn` copies the environment and runs a goroutine; in
  the browser the JavaScript backend is the reference.
- Storage needs no capability (origin-scoped browser state, never the file
  system or network); only `fetch` does, through the host-scoped grant
  read at run time (`HOWLFRAME_ALLOW_CAPS`). `capability.ForConstruct`
  classes `spawn` as `process`, but neither the JavaScript backend nor this
  one gates it: in the browser it is a task, not a process.
- No browser operation yields bytes: files, images and ZIP pass base64
  text, and the JavaScript backend's `fetch` resolves to text
  (`Response.text()`). So the VM's `[]byte` values do not reach M3's
  browser host; decided: base64 text at the boundary, as the JavaScript
  backend, and no byte value type in this milestone.
- The raw operations (`dom_query`, `set_text`, `set_html`, `set_attr`,
  `toggle_class`, `dom_value`, `on_event`) fail on a missing element with
  the engine's `TypeError` ("Cannot set properties of null"), not a
  HowlFrame error; every other element operation reports `TYPE_ERROR: op
  expected an element`. "The same errors" for these needs the JavaScript
  backend fixed first (a bugs.md entry).
- Every JavaScript backend function is `async` and every call is awaited,
  so the reference interleaves tasks at every call. Programs that do not
  race tasks against each other see no difference from a backend that only
  yields at asynchronous operations.
- Host text operations: operations HowlFrame implements in its standard
  library (`to_lower`, ZIP's duplicate check) follow HowlFrame's Unicode
  15.0.0 table, as the VM; operations the host performs (selector matching,
  attribute and class names, `encodeURIComponent`) follow the engine, since
  they are the browser's own behavior. ZIP duplicate paths therefore stay as
  M2 left them, documented.

### Browser host

A `web_app` builds with `--target wasmgc` (host `browser`) into `app.wasm`
and `app.js`, a classic script that instantiates the module against the
page and runs its top level. Decided in M3:

- Imports. Each browser host operation the program calls is one import,
  named for the operation, from a module named for its capability
  (`hf_host` for none, `hf_cap_<capability>` otherwise; the loader supplies
  a capability's module only when `HOWLFRAME_ALLOW_CAPS` grants it, so an
  ungranted program fails to instantiate; no operation in the table needs
  one yet: `fetch`, which does, comes later). Every import takes and returns
  HowlFrame values (`eqref`); `print`, `stderr`, `exit_at` come from `hf`.
- One implementation. The loader calls the JavaScript backend's own runtime
  function for each operation (`javascript.HostRuntime`), so the two
  backends share the host code and its errors. The loader includes only the
  runtimes the program's operations need (17 KB for HowlScene HF's site).
- Values. A page object (element, event, subscription) is a `$Host` holding
  an `externref`. Text crosses through the module's exported memory as
  UTF-8 (`__hf_str_out`, `__hf_str_in`). To JavaScript: nil is `null`,
  lists arrays, dicts plain objects. From JavaScript: a whole number that
  is a safe integer is an int and any other number a float; arrays are
  lists; plain objects dicts; anything else a page object; the table's
  result kind decides where it matters (elements, events, subscriptions).
  A program that can receive a number from the page includes the float
  module (`makesFloats`).
- Errors. A host function that throws sets the pending error, its message
  as `CODE: message` (a message without a code is `RUNTIME_ERROR` naming
  the engine's error), so `try_let` catches it as any error. Uncaught, the
  loader rethrows the host's own error object, so the page sees what the
  JavaScript backend's page sees. `exit` throws the JavaScript backend's
  exit error (`EXIT n at file:line`), which no `try_let` catches.
- Test blocks are left out of a page, as from the JavaScript backend's
  `app.js`; they build with `--tests`.
- The module's bytes come from `howlFrameWasm` when the page (or a test)
  defines it, otherwise from `app.wasm` beside the script. The script's
  value is a promise that settles when the top level has run.

### Handlers

A lambda (only ever the last argument of `on_event`, a subscription or
`spawn`) is lifted to a function of the module taking `(env, a0, a1)`; a
closure is a `$Closure` of a function reference and its environment, an
array of `$Cell`s. As in JavaScript, a handler shares the variables it
captures: a variable any lambda in a function names lives in a cell, made
afresh at each binding, so a handler made in a loop sees that iteration's
variable and a `set` anywhere is seen everywhere. The host calls a handler
with `__hf_invoke`. The loader turns a closure into a JavaScript function
that runs it as a task of its own and returns a promise, as the JavaScript
backend's `async` handlers do: the task's uncaught error or exit rejects
that promise and nothing else (an unhandled rejection, as there).
`on_event` uses the JavaScript backend's `howlFrameOnEvent`, the
subscriptions its `howlFrameOnMedia`, `howlFrameOnVisible` and
`howlFrameOnScroll`, and a lambda's parameters convert by their kinds in the
table (an event and an element are page objects, `matches` and `visible`
bools). `spawn` is a browser import that runs the closure at once; it needs
no capability (it is a task, not a process; `capability.ForConstruct` names
`process` for the VM's goroutine). A lambda of more than two parameters, and
`spawn` outside the browser, fail closed.

Differences from the JavaScript backend, by design (the VM is the
reference for the language): a caught error is bound to the VM's report
(`{"phase":"runtime","code":...}`) where the JavaScript backend binds an
`Error` (so `(to_string err)` reads `Error: CODE: message` there), and
`print` of a list or dict prints the VM's text (`[a b]`) where
`console.log` of an array prints `a,b`. The test page compares caught
errors by code and message.

### M4 to M6 tasks: the application

- [ ] M4: HowlScene HF's published-site runtime built with this backend;
      motion and story tests pass; size measured against 250 KiB.
- [ ] M5: the editor built with this backend; the full browser suite and
      the beginner journey pass; HowlScene HF drops `--target js`.
- [ ] M6: styles and the editor page shell generated from `.howl`; source
      policy allows only `.howl` under `src/`.

### What HowlScene HF uses

The order of M1 and M3 work follows what the application needs.

| Part | Constructs |
| --- | --- |
| Core (schema, registry, renderer, backups) | values, `let`/`set`/`if`/`for`/`do`/`return`, `defun`/`call`, modules, lists, dicts, string operations, `to_string`, `encode_json`, `parse_json`, `regex_match`, `html_escape`, `attr_escape`, `try_let`/`catch`, `is_nil`, the core-library sugar, `sniff_media_type`, `base64_text` |
| Published-site runtime | the above plus DOM queries and attributes, classes, style variables, layout reads, media queries, visibility and scroll subscriptions, preferences, `on_event`, `lambda` |
| Editor | the above plus dialogs, form values, focus, storage, file input, image decode and encode, ZIP build and read, downloads, rich text, `spawn`, `sleep` |

### Findings so far

- The bytecode VM did not scope `let`: an inner binding overwrote an outer
  variable of the same name (prints `2 2` where the interpreter, the
  JavaScript backend and this backend print `2 1`). Fixed in the VM's
  compiler first (bugs.md #58, howlframe#249); the fixture now passes.
- `%` and `//` are not VM instructions: the desugarer rewrites them into
  exact integer code that uses lists, string operations and `to_int`.
  Most list and string operations are likewise sugar over a few VM
  primitives, so this backend implements only the primitives.
- `list_get` out of range gives `""` on the VM (not nil); its index may be
  any value whose text is a whole number, though the checker only admits
  integers.
- Node validates functions lazily, so a module can run in Node and still
  be invalid; `wasm-tools validate` caught two such runtime bugs.
- VM semantics confirmed by probes: checked integer overflow is an error;
  `/` always divides as floats; division by zero is an error for floats
  too; `str_len` counts characters; `map_keys` are sorted; list and dict
  values are never equal, not even to themselves (`BcValuesEqual`); float text is Go's `%v` (exponent form below
  1e-4 and from 1e+06).
- Dicts on the VM (probed): every key is `fmt.Sprint` of the key value, so
  a runtime key `1`, `"1"` or `1.0` is the key `1` and nil is `<nil>`; the
  checker admits only string keys statically, so other keys arrive only
  through values of unknown type. A later duplicate in a literal wins.
  `map_keys` sorts in byte order (`sort.Strings`). `map_get` of a missing key
  is nil, of a non-dict a `TYPE_ERROR` naming its Go type (`int64`,
  `[]interface {}`, `<nil>`, ...). Dicts are references (a function's
  `map_set` is seen by its caller) while lists stay values. `for` over a
  dict is an error. `map_has` is desugared to a scan of `map_keys` with `=`.
- Every sugar form reduces to primitives this backend already had once
  dicts landed. The I/O and host sugar (`lines`, `read_text`, `read_json`,
  `write_json`, `sget`, `sput`, `sinc`, `unix_now`, `http_*`) waits for the
  host imports it reaches.
- The checker rejected `(sum (list 1.5 2.0))`: the desugared accumulator
  started at the integer 0 (bugs.md #60, fixed; the accumulator now takes
  the element type).
- The JavaScript backend compiled `=` to `===`, so a list or dict equalled
  itself there while the VM and the interpreter say false (bugs.md #59,
  fixed: it follows the VM's rule now).
- The VM's JSON error report escapes `<` and `>` (`\u003cnil\u003e`), so the
  differential harness compares decoded messages.
- Errors on the VM (probed): every failure in a `try_let` binding is caught,
  including in called functions, but not `exit` (so not `min` of an empty
  list, which prints and exits) and not failures in the handler or body. A
  condition that is not a boolean is `TYPE_ERROR: expected boolean, got
  <type>` (nil included); this backend treated nil as false and other
  values as true. Float `+`, `-`, `*` whose result is infinite or NaN is
  `RUNTIME_ERROR: floating-point overflow: a op b is not finite` and such a
  `/` is `division produced an invalid floating-point result`; this backend
  printed `+Inf`. Comparisons involving NaN are false.
- The VM's `%q` (in conversion messages) escapes non-printable runes, such
  as U+2028, with `strconv.IsPrint`; this backend's `quote` escapes only
  quote, backslash, newline, tab and return, so such a rune differs. Its
  loop also stopped at the first escaped character, cutting the message
  short; fixed with a fixture (`quoted in message`).
- Modules: a used file may not itself `use` another ("real transitive
  module linking is not yet supported"), on every target; HowlScene HF
  imports only from its top-level files, so this does not block it. A
  `use` path is joined to the using file's directory, so an absolute path
  loses its leading slash and fails as a missing file.
- Building each HowlScene HF core module as a `cli_app` (2026-10-29):
  `registry.howl` and `export.howl` compiled first; `schema.howl` and
  `backup.howl` stopped at `encode_json`, `render.howl` at `regex_match`.
  With the standard library all but `backup.howl` compile; it needs
  `base64_text`, which with `sniff_media_type` is a browser host operation
  of the JavaScript backend with no VM reference, so both move to M3.
- The renderer needs a few million VM instructions for one fixture
  project, far past the VM's default 100,000; the gate runs the VM with
  `run --max-instructions 500000000`.
- Lists are values, so an algorithm that updates a table in place (the
  regular-expression simulation's thread lists and marks) cannot be
  written efficiently with HowlFrame's collections: `append` to a list
  held elsewhere copies it, and there is no list update by index. Std
  code uses the `__hf_array` intrinsics; whether programs need a mutable
  array is open.
- Standard-library code needs to branch on a value's run-time type, which
  HowlFrame source cannot do (the checker types values statically and
  there is no `type_of`); `__hf_type` gives std code the VM's `%T` name.
  Whether programs should get such an operation is open.
- Running programs on the JavaScript backend as the reference for base64
  found two divergences of that backend from the VM, both fixed:
  `encode_json` kept insertion order and did not escape `<`, `>`, `&`
  (bugs.md #64; the backup gate now compares bytes), and a `try_let` used
  as a value generated JavaScript that did not parse (bugs.md #65).
- Text that is not valid UTF-8 (made with `bytes_to_string`) exposed two
  divergences that were unreachable before: `str_split` with an empty
  separator took a character's length from its lead byte, so `0xFF`
  swallowed the next three bytes (Go splits each invalid byte alone), and
  the Node loader decoded output as UTF-8 before writing it, turning
  invalid bytes into U+FFFD where the VM writes them as they are.
- `test` blocks (M2 probe, 2026-10-30). On the JavaScript backend each
  test is a `node:test` test; an uncaught error or exit fails that test
  and the rest still run; a used module's tests run where the `use` is; the
  top level is not run; exit status 1 on a failure. Two divergences from
  the VM were found and fixed first: arithmetic and ordering on a value of
  run-time type let JavaScript coerce (`(+ 1 v)` with `v` = `"x"` is
  `1x`; bugs.md #66), and a `try_let` caught a test's `exit`, so an assert
  failing inside a `try_let` binding passed (bugs.md #67). On the Go
  backend a failing assert calls `os.Exit(1)` in the `go test` binary, so
  the remaining tests never run, and `server_test.go` redeclares the
  helpers `server.go` already defines in the same package, so a test file
  that uses `map_get`, `/` or other helpers does not compile; tests whose
  names differ only in punctuation became one function (bugs.md #68,
  fixed: each failure fails its own test now).
- The host no longer computes anything for a program: float digits and
  float parsing moved from the loader (`float_digits`, `float_point`,
  `parse_float`) to `std/float.howl`, so the only imports are `print`,
  `stderr` and `exit`. The loader's float syntax had diverged already: it
  refused `1_000`, which `strconv.ParseFloat` (and so the VM) reads.
- HowlFrame's `//` and `%` desugar into list and string code, and it has
  no bit operations, so integer-heavy std code (the float module's
  arbitrary-precision decimal) uses integer intrinsics.
- ZIP (2026-10-30). The JavaScript backend's `zip_read` is asynchronous
  (it inflates through `DecompressionStream`); in this backend it is an
  ordinary call, since the inflate is HowlFrame code. Data after a deflate
  stream's final block is `ZIP_CORRUPT`, as the Compression Streams
  standard has `DecompressionStream` reject it. Node rejects it from
  22.23 (CI's runner) and accepts it before (22.22 here), so the JavaScript
  backend's result depends on the host; the `deflate junk` fixture pins the
  standard's result and compares with the JavaScript backend only where its
  Node agrees. Paths are compared for duplicates in lower
  case: JavaScript's `toLowerCase` uses full case mapping at the engine's
  Unicode version (U+0130 to `i` + U+0307, a final sigma to ς by its
  context, and Unicode 17.0 in Node 22.22) where `to_lower` is simple
  mapping at Unicode 15.0.0 (bugs.md #69). Measured against Node 22.22,
  lowering differs on U+0130, a final sigma, and the 55 code points Unicode
  16 and 17 gave a lower case, so two names that differ only there are
  duplicates on one backend and not the other. Not matched: the reference
  varies by browser engine, and matching one engine would need a second,
  newer table and the Final_Sigma rule's Cased and Case_Ignorable
  properties. M3 decides whether host text operations follow the engine
  or HowlFrame's table. The standard library gained
  `__hf_xor` (CRC-32 and stored block lengths); a std module that maps
  case or makes floats now loads the unicode or float module itself
  (before, only the program's own use did, and a non-ASCII entry name
  reached an `unreachable` bridge); and a std module with text after its
  module form is refused (`TestStdModulesParse`): a stray parenthesis had
  ended `zip.howl` early and silently dropped the functions after it.
  Speed: reading a 1 MB deflated entry or building a 550 KB archive takes
  about 1.3 s in Node, where every byte is a boxed integer in a list.
  Building a long string by repeated concatenation is quadratic, and two
  built-ins did it: `str_repeat`'s sugar appended one copy at a time
  (550 KB took 39 s), and the runtime's `str_join` appended each part to
  the result so far, which made every std module that joins parts
  quadratic too (`to_lower` of 200 KB of accented text took 59 s). Fixed
  (2026-10-30): `str_join` measures the parts and copies each once, and
  `str_repeat` counts its copies as before and builds them by doubling
  (O(log n) concatenations, about 4n bytes), so both runs take under a
  second (`TestLongTextIsLinear`, which takes 37 to 61 s per case without
  the fix). The sugar is shared, so the VM gains too: under its default
  limits `str_repeat` of `"x"` now runs to 7,654 copies (5,881 before) and
  of a 10-character piece to 7,654 (2,111 before, when its 64 MB budget,
  which counts every allocation, ran out); its output is unchanged. The
  VM's `append` still copies the list, so long lists built one item at a
  time stay quadratic against that budget there (improvements.md #111).

### Open questions

- Integer overflow and float formatting: match the VM exactly; collect
  edge cases (large values, negative zero, NaN) as fixtures.
- Asynchronous operations: resumable frames or JSPI (M3).
- Size: measure each milestone's output; the published-site runtime has a
  250 KiB budget.
- Caught error text: should `try_let` expose the error's code and message
  apart from the VM's JSON report (for example `catch` binding a dict), so
  programs can read them on every engine without parsing JSON? Until then
  the report minus its VM-only fields is the contract.
- Resource limits: the VM stops a program at an instruction limit (100,000
  by default; `--max-instructions`). This backend has no limit yet. Decide
  whether WebAssembly programs get fuel (a counter on loops and calls) or
  limits stay a VM run policy. Fixtures stay within the VM's default.

### Resuming

1. Read the status table and the first unticked task, and the milestone's
   journal in `docs/journals/` (`2026-10-30_wasmgc_m2_journal.md` for M2).
2. `go test ./internal/backend/wasmgc/ ./tools/difftest/` to see the
   current state.
3. Work test first; update this plan in the same pull request.

### Verification

Before merging a change to this backend, run the regression gate on the
final tree (after any merge of main):

    HOWLSCENE_HF=<howlscene-hf checkout> scripts/wasmgc_gate.sh

It runs `go test ./...` on the default toolchain and on Go 1.27.2, and the
HowlScene HF gates. It fails when node, wasmtime, wasm-tools or the checkout
is missing, and when a WebAssembly test skips for one of them, since a
skipped leg passes while testing less. CI's `build` job installs pinned
wasmtime and wasm-tools, so the wasmtime legs and module validation run on
every pull request. The HowlScene HF gates run only locally (the repository
is private).
