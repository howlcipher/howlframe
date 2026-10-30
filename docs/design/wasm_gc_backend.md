# WebAssembly GC backend

Status: milestone M1 exit gate met (2026-10-30); finishing its last tasks (base64, hexadecimal floats); M2 next.

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
| M1 core language | Exit gate met (2026-10-30); last tasks in progress | `build --target wasmgc` compiles `cli_app` programs using values, control flow, functions, modules, strings, lists and dicts, with `try_let`, the core-library sugar and a standard library written in HowlFrame (JSON, regular expressions, UTF-8, floats). 365 differential fixtures and the corpus (`TestWasmGCCorpusParity`, 17 programs) match the VM in Node and, built with `--host wasi`, in wasmtime; HowlScene HF's schema and renderer produce byte-identical output on both (`TestHowlSceneGate`). Unicode case mapping and trim, `bytes_to_string` and `utf8_valid` done; base64 and hexadecimal floats remain. |
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
      hard cases (halfway, boundaries, 800+ digits). Hexadecimal floats
      (`0x1p-2`) fail with `WASM_UNSUPPORTED`.
- [x] `parse_json` in `std/json.howl`, as the VM's (Go's `json.Decoder`
      with `UseNumber`, then a check for a second value): ints when they fit
      int64, otherwise floats; the last duplicate key wins; U+FFFD for
      invalid UTF-8 and unpaired surrogates; Go's syntax messages with its
      quoting of the offending byte, `unexpected EOF`, `EOF`, the
      second-value messages, and float range errors reported after them
      (`TestDiffParseJSON`). A byte list as the text fails with
      `WASM_UNSUPPORTED`. The messages are those of Go 1.25, which releases
      are built with; newer Go words some differently (bugs.md #63).
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
      from Go's `unicode.CaseRanges` (`TestUnicodeTableIsCurrent` keeps it
      in step; a toolchain of another Unicode version skips it), with the
      ASCII fast path in the runtime; `str_trim`, `to_int` and `to_float`
      trim Go's `unicode.IsSpace` set. Nil text is `<nil>` (`<NIL>` upper
      case), as `fmt.Sprint` gives it; this backend gave "" before.
      `TestDiffUnicodeText` sweeps eight long-assigned blocks, which match
      under Go 1.25 (Unicode 15.0) and 1.27 (Unicode 17.0) alike.
- [x] `bytes_to_string` (a list of byte values to a string, each as Go's
      `byte(float64(n))` on amd64) and `utf8_valid` (`std/utf8.howl`,
      Go's `utf8.ValidString`). `bytes_to_string` is how a program makes
      text that is not valid UTF-8, so `TestDiffBytes` also runs such text
      through printing, JSON, splitting, case mapping, regular expressions
      and escaping. The VM's `[]byte` values (from files and the network)
      arrive with M3's host operations.
- [ ] base64: `base64_text` and `sniff_media_type`, JavaScript-backend
      host operations with no VM form, compared with the JavaScript backend.
- [ ] Hexadecimal floats in `to_float` (Go's `atofHex`).
- [x] Exit gate, rendering: HowlScene HF's registry, schema and renderer
      compile; rendering its fixture projects gives byte-identical output
      on the VM and on WebAssembly (21,872 bytes for the two neutral
      fixtures, rendered two ways and re-encoded). `TestHowlSceneGate` runs
      it when `HOWLSCENE_HF` names a checkout; evidence in
      `docs/journals/2026-10-29_howlscene_hf_wasmgc_m1_gate.md`.
- [x] Exit gate, engines: the corpus runs identically on the VM, in Node
      and in wasmtime (2026-10-30).

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
- The JavaScript backend compiles `=` to `===`, so a list or dict equals
  itself there while the VM and the interpreter say false (bugs.md #59).
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
- Text that is not valid UTF-8 (made with `bytes_to_string`) exposed two
  divergences that were unreachable before: `str_split` with an empty
  separator took a character's length from its lead byte, so `0xFF`
  swallowed the next three bytes (Go splits each invalid byte alone), and
  the Node loader decoded output as UTF-8 before writing it, turning
  invalid bytes into U+FFFD where the VM writes them as they are.
- The host no longer computes anything for a program: float digits and
  float parsing moved from the loader (`float_digits`, `float_point`,
  `parse_float`) to `std/float.howl`, so the only imports are `print`,
  `stderr` and `exit`. The loader's float syntax had diverged already: it
  refused `1_000`, which `strconv.ParseFloat` (and so the VM) reads.
- HowlFrame's `//` and `%` desugar into list and string code, and it has
  no bit operations, so integer-heavy std code (the float module's
  arbitrary-precision decimal) uses integer intrinsics.

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

1. Read the status table and the first unticked task.
2. `go test ./internal/backend/wasmgc/ ./tools/difftest/` to see the
   current state.
3. Work test first; update this plan in the same pull request.
