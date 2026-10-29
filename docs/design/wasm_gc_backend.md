# WebAssembly GC backend

Status: design, milestone M1 in progress.

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
| dict | `$Dict` struct: insertion-ordered keys and values arrays, length, and an open-addressing index added when a dict outgrows a linear scan |
| closure (`lambda`) | `$Closure` struct `(field (ref $Fn)) (field (ref $Items))` (code and captured environment) |
| host object (DOM element, event, subscription) | `$Host` struct `(field externref)` |

Integers stay boxed in M1; type hints let a later optimization keep
`int`/`float` locals unboxed.

Printing follows the VM (`fmt.Sprint`): integers in decimal, floats in
Go's shortest form, `true`/`false`, nil as the empty string, lists as
`[a b]`, dicts as `map[k:v ...]` with sorted keys.

## Functions and calls

Each `defun` becomes a WebAssembly function taking and returning `eqref`;
arity is checked at compile time. Modules (`use`) are compiled into the same
WebAssembly module with their names prefixed, so cross-module calls are
direct calls. Lambdas are lifted to functions with an environment array;
captured variables that are assigned (`set`) are boxed in a one-slot cell.

## Error propagation

Each function returns its value and leaves a global `$err` null on success.
After any call or operation that can fail, the caller tests `$err`; if set,
it returns at once (or jumps to its `try_let` handler, which clears `$err`
and binds the message). The top level reports an uncaught error on standard
error with the VM's message and exits 1.

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
| M1 core language | In progress | Encoder and builder done; runtime values, errors, numbers, strings (literals, concatenation, comparison), printing, conversions, control flow and functions done; `build --target wasmgc` writes `app.wasm` and `app.mjs`. Differential tests run both sides through the compiler binary. Lists and string primitives done; `%`, `//`, `stderr` and `exit` work. 58 differential fixtures pass. Next: dicts, then the core-library sugar, `try_let` and modules. |
| M2 tests | Not started | |
| M3 browser host | Not started | |
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
      `fmt.Sprint` (float digits from the host for now), `to_int`,
      `to_float`, `to_string`.
- [ ] Printing of lists and dicts (`[a b]`, `map[k:v]` with sorted keys).
- [x] `build --target wasmgc` (named `wasm` once it covers the legacy WAT
      target): writes `app.wasm` and a generated loader (`app.mjs`) that
      runs a `cli_app` in Node with print as a host import. Arguments and
      exit to follow.
- [ ] `tools/difftest` target `wasm`, run in Node and in wasmtime.
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
      Gap: `to_lower`, `to_upper` and `str_trim` handle ASCII only (the VM
      uses Go's Unicode tables).
- [ ] Constructs, in this order, each with fixtures: dicts (`dict`, `map_get`, `map_set`, `map_has`, `map_keys`,
      `map_delete`); the core-library sugar after desugaring (`filter`,
      `map_list`, `reduce`, `sort` and the rest); `try_let`, `catch`, `exit`,
      `stderr`; `is_nil`; modules (`use`, `export`).
- [ ] Standard library in HowlFrame (`std/`), on byte intrinsics:
      `encode_json`, `parse_json`, `regex_match` (the regular expression
      subset the corpus uses, documented), `html_escape`, `attr_escape`,
      `utf8_valid`, `bytes_to_string`, base64.
- [ ] Exit gate: HowlScene HF's registry, schema and renderer compile;
      rendering a fixture project gives byte-identical HTML on the VM and
      on WebAssembly.

### M2 tasks: tests

- [ ] `test` blocks and `assert` compile into a test module with one export
      per test; a generated Node runner reports results like `node:test`.
- [ ] HowlScene HF's `tests/core/*_test.howl` pass on WebAssembly.

### M3 tasks: browser host

- [ ] Typed host operation table (name, parameter and result kinds,
      capability, synchronous or asynchronous) shared by the checker and the
      backends; the browser loader is generated from it.
- [ ] Host object handles (`$Host` with `externref`).
- [ ] Closures and event dispatch: `lambda`, `on_event`, `spawn`, and the
      subscription operations (`on_media`, `on_visible`, `on_scroll`, `off`).
- [ ] Asynchronous operations (storage, files, image decode and encode, ZIP,
      `sleep`): resumable frames, decided with measurements; JSPI where
      available is the alternative.
- [ ] Every DOM, storage, file, image, archive and rich text operation the
      JavaScript backend supports, with the same errors.
- [ ] HowlFrame's browser tests (`internal/backend/javascript/browser_*`)
      ported to run against both backends.

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
  equality is identity; float text is Go's `%v` (exponent form below
  1e-4 and from 1e+06).
- Temporary host dependencies, to be replaced by standard-library
  HowlFrame: shortest float digits (`float_digits`, `float_point`) and
  float parsing (`parse_float`, `parse_float_ok`).

### Open questions

- Dict key types: the VM accepts which key types, and how does it order
  `map_keys`? To settle with fixtures before implementing dicts.
- Integer overflow and float formatting: match the VM exactly; collect
  edge cases (large values, negative zero, NaN) as fixtures.
- Asynchronous operations: resumable frames or JSPI (M3).
- Size: measure each milestone's output; the published-site runtime has a
  250 KiB budget.
- Resource limits: the VM stops a program at an instruction limit (100,000
  by default; `--max-instructions`). This backend has no limit yet. Decide
  whether WebAssembly programs get fuel (a counter on loops and calls) or
  limits stay a VM run policy. Fixtures stay within the VM's default.

### Resuming

1. Read the status table and the first unticked task.
2. `go test ./internal/backend/wasmgc/ ./tools/difftest/` to see the
   current state.
3. Work test first; update this plan in the same pull request.
