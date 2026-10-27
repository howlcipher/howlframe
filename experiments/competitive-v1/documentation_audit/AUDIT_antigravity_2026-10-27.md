# HowlFrame Documentation Audit Report (Re-Audit)

This audit re-evaluates the HowlFrame documentation in `docs/` (`README.md`, `SKILL.md`, `docs/cli.md`, and `docs/reference/*`) against the compiler binary `bin/howlframe` (version 0.1.0, built from HowlFrame main commit `ee8dd445`, HFBC format 2) for the competitive-v1 experiment (protocol gate 3).

All audit programs and commands are stored under `audit_programs/`. No files under `docs/` have been modified.

---

## 1. Discrepancy Table

| File:Line | What Documentation Says | What Compiler Does | Severity |
|---|---|---|---|
| `docs/README.md:210-213,360,456` | Claims running `go run howlframe.go -o build examples/wasm_math.howl` compiles a `wasm_app` and "writes `build/app.wat`". Line 456 claims: "WebAssembly targets write `app.wat`." Line 467-468 adds that `-o <dir>` creates the directory and writes legacy WAT. | Running `howlframe -o build examples/wasm_math.howl` defaults to standalone bytecode compilation and writes `build/wasm_math.hfbc` (a binary bytecode artifact), not `build/app.wat`. Compiling to WebAssembly requires `--target wasm` or `-compile-wasm`. Furthermore, running `howlframe build <source.howl> --target wasm` without `-o` defaults to writing `<input>.ssa.wat` (for example `test_wasm_math.howl.ssa.wat`), not `app.wat`. | **breaks** |
| `docs/docs/reference/process_authority.md:33,68` | Recommends `-allow-caps 'process:exec=/usr/bin/printf %s *'` (line 33) and `(spawn_agent "Name" (grants "environment" "process:exec=/usr/bin/printf **") ...)` (line 68). | On modern Linux systems where `/usr/bin/printf` is a symlink (such as modern usr-merged Ubuntu with rust/cargo coreutils, where `/usr/bin/printf -> ../lib/cargo/bin/coreutils/printf`), attempting to execute `/usr/bin/printf` is rejected at runtime with `{"phase":"runtime","code":"CAPABILITY_DENIED","function":"main","instruction":3,"opcode":"EXEC","message":"capability denied: process: exec program is or traverses a symlink; grant and call its canonical path"}`. Following the documented example directly fails. | **breaks** |
| `docs/README.md:553-560` | States: `Prove locally, from a repository checkout, with:` followed by `go build -o howlframe howlframe.go`, `python3 benchmarks/language_write_cost/recount_and_verify.py`, `python3 benchmarks/v2/harness/prove_howlframe_refs.py`, and `python3 benchmarks/real_apps/prove_real_apps.py`. | The `benchmarks/` directory is not included in the distribution workspace bundle. Attempting to execute any of these commands fails immediately with `[Errno 2] No such file or directory`. | **breaks** |
| `docs/docs/cli.md:85` | States: `list_len, list_get, for and bytes_to_string use the bytes as they are; str_join and append build a list from them and charge 8 bytes per element. bytes_to_string charges the text again, and str_split, str_len and str_slice charge every part they split the text into.` | Contains multiple direct contradictions and inaccuracies: (1) It directly contradicts itself regarding `bytes_to_string`, stating both that it "uses the bytes as they are" (0 charge) and that it "charges the text again". Testing confirms instruction `CONVERT` (`bytes_to_string`) charges 1 byte per byte of the string. (2) It claims `str_join` "builds a list from them and charges 8 bytes per element". In reality, `str_join` consumes a list and produces a string, charging 1 byte per byte of the resulting joined string, not 8 bytes per element. (3) It claims `append` "charges 8 bytes per element". In reality, appending reallocates the underlying slice and charges the newly allocated slice capacity (e.g., 16 bytes for 2nd element, 24 bytes for 3rd element). | **misleads** |
| `docs/README.md:296-298` | States: `This is a JSON-only route-input path; it does not provide a general request-body value, streaming input, or a configurable body-size limit.` | HowlFrame DOES provide a general request-body string accessor via `(req_body req)` (documented in `SKILL.md:78,109` and opcode `HTTP_REQ_BODY`), and DOES provide a configurable request body-size limit via `--max-request-bytes` (default 1,048,576 bytes; documented in `docs/cli.md:32`). Stating that it does not provide a general request-body value or configurable body-size limit is outdated and incorrect. | **misleads** |
| `docs/docs/reference/construct_coverage.md:8,34,38,43,66,96,116,130,143,149,157,159,162,183,190` | Coverage table lists `%`, `cond`, `dec`, `ends_with`, `inc`, `not`, `rec`, `reverse`, `sort`, `starts_with`, `str_contains`, `str_len`, `str_slice`, `unless`, and `when` as `Standalone Bytecode: No` and `Compile-Time Only?: Yes`. | All of these constructs compile, build, and execute cleanly on the standalone bytecode target (`build` and `run`) via compiler frontend desugaring and lowering. Claiming that standalone bytecode does not support them misleads application authors into believing standard language features are unavailable on the bytecode VM. | **misleads** |
| `docs/README.md:438-444` | Suggests running Ollama and orchestrator with: `ollama serve`, `ollama pull llama3`, and `python tools/orchestrator/orchestrator.py`. | Neither Ollama nor the `tools/orchestrator` directory are present in the offline distribution workspace, and network access is disabled. | **misleads** |
| `docs/docs/reference/lowered_hfir_abi_v1.md:62` | States: `Integer / is not one rule yet. The interpreter truncates int64. The bytecode VM divides float64.` | Under the unified numeric contract (`NUMERIC_CONTRACT.md`), `/` is always real division across all targets. The interpreter (`-run` and `run --target interpreter`) evaluates `(/ 9 2)` to `4.5` (`float64`), exactly matching the bytecode VM; it does not truncate `int64`. | **misleads** |
| `docs/docs/cli.md:104-106` | Documents: `-compile-bc : Compile AST to bytecode JSON`, `-compile-hfir-bc : Compile directly from semantic HFIR to bytecode JSON`, and `-run-bc : Run bytecode from JSON file`. (Repeated in `bin/howlframe --help`). | `-compile-bc` and `-compile-hfir-bc` emit binary format 2 `.hfbc` / `.bc.bin` files (magic bytes `HFBC\x02\x00\x00\x00` followed by gob payload), NOT JSON. | **cosmetic** |
| `docs/docs/cli.md:15` | Documents default artifact filenames for Go and JS: `--target go writes server.go (and server_test.go when the program has tests), and --target js writes app.js (and app.test.js), into the current directory or the directory given with -o; an -o path ending in .go or .js writes only that file.` | Omits `--target wasm` default artifact name from this list. `--target wasm` writes `<input>.ssa.wat` (or the file given with `-o`), not `app.wat`. | **cosmetic** |

---

## 2. Summary by Severity

* **Breaks:** 3
* **Misleads:** 5
* **Cosmetic:** 2
* **Total Discrepancies:** 10

---

## 3. Re-test of Previous 20 Findings from PREVIOUS_AUDIT.md

Each finding from `PREVIOUS_AUDIT.md` (2026-10-25) was re-tested against the current documentation in `docs/` and the compiler binary (`bin/howlframe`, version 0.1.0, commit `ee8dd445`).

### Finding 1: `docs/SKILL.md:214` — Recommended `howlframe compile-js`
* **Status:** **Fixed**
* **Evidence:** In `docs/SKILL.md:215`, the canonical build script was updated to:
  ```bash
  howlframe build app/frontend.howl --target js -o static/app.js
  ```
  Tested by compiling `audit_programs/test_finding_1.howl`:
  ```bash
  bin/howlframe build audit_programs/test_finding_1.howl --target js -o audit_programs/test_finding_1.js
  ```
  Compiler output: `Built audit_programs/test_finding_1.js` (exit code 0).

### Finding 2: `docs/SKILL.md:229` — Recommends `-caps` flag for `howlframe run`
* **Status:** **Fixed**
* **Evidence:** In `docs/SKILL.md:230`, the checklist was updated to:
  ```text
  Runtime grants (howlframe run --allow-caps network,database,filesystem ...) match application requirements...
  ```
  Running with legacy `-caps` fails:
  ```bash
  bin/howlframe run -caps network audit_programs/hello.howl
  ```
  Compiler output: `flag provided but not defined: -caps` (exit code 2).
  Running with `--allow-caps network audit_programs/hello.howl` succeeds with exit code 0.

### Finding 3: `docs/SKILL.md:148-173` — Untyped `defun` in `web_app` returning void
* **Status:** **Fixed**
* **Evidence:** In `docs/SKILL.md:151`, the function now explicitly declares:
  ```lisp
  (type_hint return "void")
  ```
  Tested with `audit_programs/test_finding_3.howl`:
  ```bash
  bin/howlframe check audit_programs/test_finding_3.howl
  bin/howlframe build audit_programs/test_finding_3.howl --target js -o audit_programs/test_finding_3.js
  ```
  Compiler output: `OK: audit_programs/test_finding_3.howl` and `Built audit_programs/test_finding_3.js` (exit code 0).

### Finding 4: `docs/SKILL.md:47` — Module calls using dot syntax
* **Status:** **Fixed**
* **Evidence:** `docs/SKILL.md:47` now states:
  ```text
  Call an exported function as (call alias/func args...), with a slash; alias.func is not a module call.
  ```
  And `docs/README.md:386` states:
  ```text
  check rejects the dotted spelling (call m.name ...) for a function the module defines...
  ```
  Tested in `audit_programs/test_mod_dot.howl`:
  ```bash
  bin/howlframe check audit_programs/test_mod_dot.howl
  ```
  Compiler output:
  ```json
  {"reason":"undefined reference to function \"m.add_one\": call a function exported by module m with a slash, (call m/add_one ...), not a dot","line":3,"column":16}
  ```
  Tested slash call in `audit_programs/test_mod_slash.howl`:
  ```bash
  bin/howlframe run audit_programs/test_mod_slash.howl
  ```
  Compiler output: `42` (exit code 0).

### Finding 5: `docs/SKILL.md:51` — `try_let` documented with 2 operands
* **Status:** **Fixed**
* **Evidence:** `docs/SKILL.md:51` was corrected to:
  ```text
  (try_let (var expr) (catch err handler) body): three operands; body runs with var bound when expr succeeds.
  ```
  Tested in `audit_programs/test_try_let_3args.howl`: exits 0 and prints `1`. Supplying only 2 operands (`audit_programs/test_try_let_2args.howl`) fails compilation with:
  ```json
  {"reason":"try_let expects 3 arguments, got 2","line":3,"column":5}
  ```

### Finding 6: `docs/README.md:173-177` — Default Go backend writing `server.go`
* **Status:** **Fixed**
* **Evidence:** `docs/README.md:176-182` was updated to state:
  ```text
  With no subcommand, the compiler checks the program and runs it directly in the bytecode VM. To generate Go instead, build with --target go, which writes server.go to the current directory, then run it:
  go run howlframe.go build examples/cli_hello.howl --target go
  go run server.go
  ```
  Tested running `bin/howlframe audit_programs/hello.howl`: runs directly in VM and prints `Hello from HowlFrame`.
  Tested running `bin/howlframe build audit_programs/hello.howl --target go`: prints `Built server.go` and writes `server.go`.

### Finding 7: `docs/README.md:330-333` — Web app generation without `--target js`
* **Status:** **Fixed**
* **Evidence:** `docs/README.md:342-349` was updated to:
  ```bash
  go run howlframe.go build counter.howl --target js
  node --test app.test.js
  ```
  And explicitly explains: `Without --target js, a web_app is rejected by the bytecode target.`
  Tested in `audit_programs/counter.howl`: `bin/howlframe build audit_programs/counter.howl --target js` writes `app.js` and `app.test.js` (exit 0). Without `--target js`, it fails closed with `HFIR_TARGET_INFEASIBLE` (exit 1).

### Finding 8: `docs/README.md:391-400` — `optimize_signature` referencing undeclared symbol
* **Status:** **Fixed**
* **Evidence:** `docs/README.md:422-430` wraps the form in a bound variable:
  ```lisp
  (cli_app
    (let (support_prompt "prompt")
      (optimize_signature support_prompt
        (metric "accuracy")
        (test "go test ./...")
        (candidate "baseline" "Answer clearly.")
        (candidate "strict" "Answer only with verified facts.")
        (print "body"))))
  ```
  Tested in `audit_programs/test_optimize.howl`:
  ```bash
  bin/howlframe -optimization-plan audit_programs/test_optimize.howl
  ```
  Compiler output:
  ```json
  {"format":"howlframe.optimization_plan/v1","signatures":[{"name":"support_prompt","metric":"accuracy","tests":["go test ./..."],"candidates":[{"label":"baseline","payload":"Answer clearly."},{"label":"strict","payload":"Answer only with verified facts."}],"line":3,"column":5,"body_type":"void"}]}
  ```

### Finding 9: `docs/README.md:522-527` (now 553-560) — Missing benchmark files
* **Status:** **Still present**
* **Evidence:** `docs/README.md:553` added `Prove locally, from a repository checkout, with:`, but the `benchmarks/` directory is not included in the distribution workspace bundle. Running `python3 benchmarks/language_write_cost/recount_and_verify.py` fails with `[Errno 2] No such file or directory`.

### Finding 10: `docs/docs/reference/NUMERIC_CONTRACT.md:26` — `to_float` returning NaN/Inf
* **Status:** **Fixed**
* **Evidence:** The compiler binary was updated to refuse NaN and Infinity. Tested in `audit_programs/test_to_float_nan.howl`:
  `to_float` with `"NaN"`, `"nan"`, `"Inf"`, `"Infinity"`, `"+Inf"`, and `"-Inf"` now all raise catchable runtime errors:
  ```json
  {"phase":"runtime","code":"CONVERSION_ERROR","function":"main","instruction":0,"opcode":"CONVERT","message":"cannot convert \"NaN\" to float"}
  ```

### Finding 11: `docs/SKILL.md:52` — `(append list item)` return value
* **Status:** **Fixed**
* **Evidence:** `docs/SKILL.md:52` now explicitly states:
  ```text
  (append list item) (a statement: it changes list in place and produces no value).
  ```
  Tested in `audit_programs/test_append.howl`: `(append l 3)` mutates `l` in place.
  Tested in `audit_programs/test_append_bind.howl`: attempting `(let (x (append l 3)))` fails compilation with:
  ```json
  {"reason":"initializer of \"x\" is a void expression and produces no value","line":3,"column":13}
  ```

### Finding 12: `docs/README.md:426` (now 458-459) — Default bytecode extension
* **Status:** **Fixed**
* **Evidence:** `docs/README.md:458-459` now distinguishes:
  ```text
  howlframe build (bytecode) writes <name>.hfbc; the legacy -compile-bc flag writes <input>.bc.bin (for example hello.howl.bc.bin).
  ```
  Tested: `bin/howlframe build audit_programs/hello.howl` writes `hello.hfbc`, and `bin/howlframe -compile-bc audit_programs/hello.howl` writes `audit_programs/hello.howl.bc.bin`.

### Finding 13: `docs/README.md:78-81` (now 83-84, and `docs/docs/cli.md:15`) — Destination directory of `build`
* **Status:** **Fixed**
* **Evidence:** `docs/README.md:83-84` and `docs/docs/cli.md:15` clarify:
  ```text
  build writes <name>.hfbc to the current directory, even when the source is in another directory; use -o <path> to choose the output path.
  ```
  Tested: `bin/howlframe build audit_programs/hello.howl` writes `hello.hfbc` in the current working directory, and `bin/howlframe build audit_programs/hello.howl -o audit_programs/custom_hello.hfbc` writes to the specified path.

### Finding 14: `docs/README.md:282-284` (now 293-296) — `parse_json` argument restriction
* **Status:** **Fixed**
* **Evidence:** `docs/README.md:293-296` now clarifies:
  ```text
  In the standalone bytecode VM, req.body is valid only as the input to parse_json in an HTTP route. Everywhere else, the input to parse_json must be a variable name, not an expression or literal: bind the text with let first.
  ```
  Tested in `audit_programs/test_parse_json_lit.howl`: passing a literal to `parse_json` fails compilation with:
  ```json
  {"reason":"parse_json body must be a variable name, not an expression or literal; bind it with let first","line":2,"column":30}
  ```

### Finding 15: `docs/docs/cli.md:1-130` (now 88-91) — Missing `inspect` documentation
* **Status:** **Fixed**
* **Evidence:** `docs/docs/cli.md:88-91` now documents `## howlframe inspect`:
  ```text
  Usage: howlframe inspect --caps <artifact.hfbc>
  Reports, without executing anything, the capabilities a compiled artifact requires and the instruction sites that need them, as compact JSON...
  ```
  Tested on `audit_programs/test_read_file.hfbc`:
  ```bash
  bin/howlframe inspect --caps audit_programs/test_read_file.hfbc
  ```
  Output:
  ```json
  {"capabilities":["filesystem"],"sites":[{"function":"main","index":1,"opcode":"READ_FILE","capability":"filesystem","reason":"opcode registry"}]}
  ```

### Finding 16: `docs/docs/cli.md:6` — Synopsis omitted `--profile`
* **Status:** **Fixed**
* **Evidence:** `docs/docs/cli.md:6` now documents:
  ```text
  Usage: howlframe check [--profile=default|governed] <source.howl>
  ```
  Tested: `bin/howlframe check --profile=governed audit_programs/hello.howl` exits 0 with `OK: audit_programs/hello.howl`.

### Finding 17: `docs/docs/reference/artifact_compatibility.md:18-20,40` (now 17-24) — Runtime v0.1.0 format version
* **Status:** **Fixed**
* **Evidence:** Table and text updated in `docs/docs/reference/artifact_compatibility.md:17-24`:
  ```text
  | Current runtime (howlframe version prints HFBC format: 2) | Supported (Backward Compatible) | Supported | Version 2 |

  The runtime version string was not changed with the format: current builds still print HowlFrame 0.1.0 together with HFBC format: 2. Use the HFBC format line, not the version number, to tell which artifacts a binary emits.
  ```
  `bin/howlframe version` outputs:
  ```text
  HowlFrame 0.1.0
  HFBC format: 2
  ```

### Finding 18: `docs/README.md:408-412` (now 438-444) — Missing orchestrator and Ollama
* **Status:** **Still present**
* **Evidence:** `docs/README.md:438` added `From a repository checkout, with Ollama installed...`, but neither Ollama nor the `tools/orchestrator` directory are present in the offline distribution workspace.

### Finding 19: `docs/docs/cli.md:13` — Synopsis omitted `--target` and `--profile`
* **Status:** **Fixed**
* **Evidence:** `docs/docs/cli.md:13` now documents:
  ```text
  Usage: howlframe build <source.howl> [-o <output>] [--target=bytecode|go|js|wasm] [--profile=default|governed]
  ```
  Matches `bin/howlframe build --help`.

### Finding 20: `docs/docs/reference/process_authority.md:32` — `/usr/bin/find` symlink caveat
* **Status:** **Not applicable on this host / Fixed**
* **Evidence:** On this testing host, `/usr/bin/find` is a regular executable ELF file (`-rwxr-xr-x+`), not a symlink. Running `audit_programs/test_exec_find.howl` with `--allow-caps 'process:exec=/usr/bin/find **,filesystem'` succeeds cleanly. (However, `/usr/bin/printf` in line 33 is a symlink on this host and fails, as documented in Discrepancy 2).

---

## 4. Test Programs and Execution Results

Below is the complete ledger of programs written under `audit_programs/` and commands executed during this audit.

### 4.1 Basic CLI and Diagnostics

#### Program: `audit_programs/hello.howl`
```lisp
(cli_app
  (print "Hello from HowlFrame")
)
```
* **Command:** `bin/howlframe check audit_programs/hello.howl`
  * **Exit Code:** 0
  * **Output:**
    ```text
    OK: audit_programs/hello.howl
    ```
* **Command:** `bin/howlframe build audit_programs/hello.howl`
  * **Exit Code:** 0
  * **Output:**
    ```text
    Built hello.hfbc
    ```
* **Command:** `bin/howlframe run hello.hfbc`
  * **Exit Code:** 0
  * **Output:**
    ```text
    Hello from HowlFrame
    ```
* **Command:** `bin/howlframe audit_programs/hello.howl` (direct VM execution without subcommand)
  * **Exit Code:** 0
  * **Output:**
    ```text
    Hello from HowlFrame
    ```
* **Command:** `bin/howlframe check --profile=governed audit_programs/hello.howl`
  * **Exit Code:** 0
  * **Output:**
    ```text
    OK: audit_programs/hello.howl
    ```
* **Command:** `bin/howlframe build audit_programs/hello.howl --target go`
  * **Exit Code:** 0
  * **Output:**
    ```text
    Built server.go
    ```
* **Command:** `bin/howlframe -compile-bc audit_programs/hello.howl`
  * **Exit Code:** 0
  * **Output File:** `audit_programs/hello.howl.bc.bin`

#### Diagnostic Stream & Error Format Checks
* **Command:** `bin/howlframe check audit_programs/test_append_bind.howl`
  * **Exit Code:** 1
  * **Stdout:** `{"reason":"initializer of \"x\" is a void expression and produces no value","line":3,"column":13}`
  * **Stderr:** *(empty)*
* **Command:** `bin/howlframe build audit_programs/counter.howl`
  * **Exit Code:** 1
  * **Stdout:**
    ```json
    [{"code":"HFIR_TARGET_INFEASIBLE","severity":"ERROR","message":"construct \"on_event\" cannot be compiled for the standalone bytecode target: no bytecode lowering exists for it","location":{"filename":"counter.howl","line":8,"column":4},"contract_version":"v1","target":"bytecode"},{"code":"HFIR_TARGET_INFEASIBLE","severity":"ERROR","message":"construct \"test\" cannot be compiled for the standalone bytecode target: test blocks are extracted into Go _test.go output; the VM cannot run them (tracked by improvements.md #96)","location":{"filename":"counter.howl","line":12,"column":4},"contract_version":"v1","target":"bytecode"}]
    ```
  * **Stderr:** *(empty)*
* **Command:** `bin/howlframe check --unknown-flag audit_programs/hello.howl`
  * **Exit Code:** 2
  * **Stdout:** `Usage: howlframe check [--profile=default|governed] <source.howl>`
  * **Stderr:** `flag provided but not defined: -unknown-flag`
* **Command:** `bin/howlframe check nonexistent_file_xyz.howl`
  * **Exit Code:** 1
  * **Stdout:** *(empty)*
  * **Stderr:** `Cannot read file: open nonexistent_file_xyz.howl: no such file or directory`

---

### 4.2 Web App and JavaScript Target

#### Program: `audit_programs/test_finding_1.howl`
```lisp
(web_app
  (defun hello ()
    (type_hint return "string")
    (return "world")
  )
)
```
* **Command:** `bin/howlframe build audit_programs/test_finding_1.howl --target js -o audit_programs/test_finding_1.js`
  * **Exit Code:** 0
  * **Output:** `Built audit_programs/test_finding_1.js`

#### Program: `audit_programs/test_finding_3.howl`
```lisp
(web_app
  (defun load_data ()
    (type_hint return "void")
    (do
      (let (status_banner (dom_query "#status"))
        (set_text status_banner "Loading...")
      )
      (try_let (raw_resp (fetch "/api/data" "GET"))
        (catch err
          (let (banner (dom_query "#status"))
            (set_text banner "Failed to load data")
          )
        )
        (try_let (parsed (parse_json DataResponse raw_resp))
          (catch json_err
            (print "JSON parse error")
          )
          (let (container (dom_query "#content"))
            (set_attr container "class" "loaded")
          )
        )
      )
    )
  )
)
```
* **Command:** `bin/howlframe check audit_programs/test_finding_3.howl`
  * **Exit Code:** 0
  * **Output:** `OK: audit_programs/test_finding_3.howl`
* **Command:** `bin/howlframe build audit_programs/test_finding_3.howl --target js -o audit_programs/test_finding_3.js`
  * **Exit Code:** 0
  * **Output:** `Built audit_programs/test_finding_3.js`

#### Program: `audit_programs/counter.howl`
```lisp
(web_app
  (defun increment (n)
    (type_hint n "int")
    (type_hint return "int")
    (return (+ n 1))
  )

  (on_event (dom_query "#btn") "click" (lambda (e)
    (set_text (dom_query "#label") (call increment 1))
  ))

  (test "increment works"
    (if (!= (call increment 1) 2)
      (print "failed")
    )
  )
)
```
* **Command:** `bin/howlframe build audit_programs/counter.howl --target js`
  * **Exit Code:** 0
  * **Output:** `Built app.js` (writes `app.js` and `app.test.js`)

#### Program: `audit_programs/test_web_app_dom_value.howl`
```lisp
(web_app
  (defun on_click ()
    (type_hint return "void")
    (let (inp (dom_query "#input"))
      (let (val (dom_value inp))
        (set_text (dom_query "#label") val)
      )
    )
  )
)
```
* **Command:** `bin/howlframe build audit_programs/test_web_app_dom_value.howl --target js -o audit_programs/test_web_app_dom_value.js`
  * **Exit Code:** 0
  * **Output:** `Built audit_programs/test_web_app_dom_value.js`

#### Program: `audit_programs/test_url_encode.howl`
```lisp
(web_app
  (defun test_enc ()
    (type_hint return "string")
    (return (url_encode "hello world & test"))
  )
)
```
* **Command:** `bin/howlframe build audit_programs/test_url_encode.howl --target js -o audit_programs/test_url_encode.js`
  * **Exit Code:** 0
  * **Output:** `Built audit_programs/test_url_encode.js`

#### Program: `audit_programs/test_url_encode_bc.howl`
```lisp
(cli_app
  (print (url_encode "hello world"))
)
```
* **Command:** `bin/howlframe build audit_programs/test_url_encode_bc.howl`
  * **Exit Code:** 1
  * **Output:**
    ```json
    [{"code":"HFIR_TARGET_INFEASIBLE","severity":"ERROR","message":"construct \"url_encode\" cannot be compiled for the standalone bytecode target: no bytecode lowering exists for it","location":{"filename":"test_url_encode_bc.howl","line":2,"column":11},"contract_version":"v1","target":"bytecode"}]
    ```

---

### 4.3 Modules and Scoping

#### Module: `audit_programs/math_mod.howl`
```lisp
(module
  (export
    (defun add_one (x)
      (type_hint x "int")
      (type_hint return "int")
      (return (+ x 1))
    )
  )
)
```

#### Program: `audit_programs/test_mod_slash.howl`
```lisp
(cli_app
  (use "math_mod.howl" as m)
  (print (call m/add_one 41))
)
```
* **Command:** `bin/howlframe run audit_programs/test_mod_slash.howl`
  * **Exit Code:** 0
  * **Output:** `42`

#### Program: `audit_programs/test_mod_dot.howl`
```lisp
(cli_app
  (use "math_mod.howl" as m)
  (print (call m.add_one 41))
)
```
* **Command:** `bin/howlframe check audit_programs/test_mod_dot.howl`
  * **Exit Code:** 1
  * **Output:**
    ```json
    {"reason":"undefined reference to function \"m.add_one\": call a function exported by module m with a slash, (call m/add_one ...), not a dot","line":3,"column":16}
    ```

#### Program: `audit_programs/test_mod_private.howl`
```lisp
(cli_app
  (use "math_mod.howl" as m)
  (print (call m/secret))
)
```
* **Command:** `bin/howlframe check audit_programs/test_mod_private.howl`
  * **Exit Code:** 1
  * **Output:** `{"reason":"undefined reference to function \"m/secret\"","line":3,"column":16}`

---

### 4.4 Flow Control, Collections, and Error Handling

#### Program: `audit_programs/test_try_let_3args.howl`
```lisp
(cli_app
  (let (s "{\"a\": 1}")
    (try_let (data (parse_json Any s))
      (catch err (print "parse error"))
      (print (map_get data "a"))
    )
  )
)
```
* **Command:** `bin/howlframe run audit_programs/test_try_let_3args.howl`
  * **Exit Code:** 0
  * **Output:** `1`

#### Program: `audit_programs/test_try_let_2args.howl`
```lisp
(cli_app
  (let (s "{\"a\": 1}")
    (try_let (data (parse_json Any s))
      (catch err (print "parse error"))
    )
  )
)
```
* **Command:** `bin/howlframe check audit_programs/test_try_let_2args.howl`
  * **Exit Code:** 1
  * **Output:** `{"reason":"try_let expects 3 arguments, got 2","line":3,"column":5}`

#### Program: `audit_programs/test_append.howl`
```lisp
(cli_app
  (let (l (list 1 2))
    (do
      (append l 3)
      (print (list_len l))
    )
  )
)
```
* **Command:** `bin/howlframe run audit_programs/test_append.howl`
  * **Exit Code:** 0
  * **Output:** `3`

#### Program: `audit_programs/test_append_bind.howl`
```lisp
(cli_app
  (let (l (list 1 2))
    (let (x (append l 3))
      (print x)
    )
  )
)
```
* **Command:** `bin/howlframe check audit_programs/test_append_bind.howl`
  * **Exit Code:** 1
  * **Output:** `{"reason":"initializer of \"x\" is a void expression and produces no value","line":3,"column":13}`

#### Program: `audit_programs/test_parse_json_lit.howl`
```lisp
(cli_app
  (let (data (parse_json Any "{\"a\": 1}"))
    (print data)
  )
)
```
* **Command:** `bin/howlframe check audit_programs/test_parse_json_lit.howl`
  * **Exit Code:** 1
  * **Output:** `{"reason":"parse_json body must be a variable name, not an expression or literal; bind it with let first","line":2,"column":30}`

---

### 4.5 Instruction Counts and Execution Receipts

#### Program: `audit_programs/test_loop_cost.howl` (10 iterations)
```lisp
(cli_app
  (let (i 0)
    (while (< i 10)
      (set i (+ i 1))
    )
  )
)
```
* **Command:** `bin/howlframe build audit_programs/test_loop_cost.howl -o audit_programs/test_loop_cost.hfbc && bin/howlframe run --receipt audit_programs/receipt_10.json audit_programs/test_loop_cost.hfbc`
  * **Exit Code:** 0
  * **Receipt Content:**
    ```json
    {
      "schema": "howlframe.receipt/v0",
      "artifact_sha256": "827223e23b7e295248a6a4709689053ea37fa5e06b9f016ed0c3aed62fa48de4",
      "compiler_version": "0.1.0",
      "grant": [],
      "limits": {
        "max_instructions": 100000,
        "max_call_depth": 1000,
        "max_memory_bytes": 67108864,
        "max_fetch_body_bytes": 10485760,
        "max_exec_output_bytes": 10485760,
        "deadline_ms": 0
      },
      "instructions_used": 96,
      "effects": [],
      "effects_truncated": false,
      "exit": {
        "status": "ok",
        "code": 0,
        "error": null
      }
    }
    ```

#### Program: `audit_programs/test_loop_cost_20.howl` (20 iterations)
```lisp
(cli_app
  (let (i 0)
    (while (< i 20)
      (set i (+ i 1))
    )
  )
)
```
* **Command:** `bin/howlframe build audit_programs/test_loop_cost_20.howl -o audit_programs/test_loop_cost_20.hfbc && bin/howlframe run --receipt audit_programs/receipt_20.json audit_programs/test_loop_cost_20.hfbc`
  * **Exit Code:** 0
  * **Receipt `instructions_used`:** `186`
  * **Result:** `(186 - 96) / (20 - 10) = 9` instructions per loop iteration, confirming the exact claim in `docs/cli.md:84`.

#### Program: `audit_programs/test_cli_args.howl`
```lisp
(cli_app
  (let (args (cli_args))
    (print (str_join args " "))
  )
)
```
* **Command:** `bin/howlframe run audit_programs/test_cli_args.howl a b c`
  * **Exit Code:** 0
  * **Output:** `a b c`
* **Command:** `bin/howlframe run audit_programs/test_cli_args.howl -- a b c`
  * **Exit Code:** 0
  * **Output:** `-- a b c` (verifying `--` is passed through verbatim as documented in `docs/cli.md:20`).

---

### 4.6 Memory Allocation and Resource Limits

#### Program: `audit_programs/test_mem_concat.howl`
```lisp
(cli_app
  (let (s (+ "hello " "world"))
    (print s)
  )
)
```
* **Command:** `bin/howlframe run --max-memory-bytes 10 audit_programs/test_mem_concat.howl`
  * **Exit Code:** 1
  * **Stderr:** `{"phase":"runtime","code":"LIMIT_EXCEEDED","function":"main","instruction":2,"opcode":"BINOP","message":"memory limit exceeded (max 10 bytes)"}`
* **Command:** `bin/howlframe run --max-memory-bytes 11 audit_programs/test_mem_concat.howl`
  * **Exit Code:** 0
  * **Output:** `hello world` (verifying 11-byte charge for 11-character string concatenation).

#### Program: `audit_programs/test_mem_read_file.howl` (reading 100-byte `data_100.txt`)
```lisp
(cli_app
  (let (b (read_file "audit_programs/data_100.txt"))
    (print (list_len b))
  )
)
```
* **Command:** `bin/howlframe run --allow-caps filesystem --max-memory-bytes 99 audit_programs/test_mem_read_file.howl`
  * **Exit Code:** 1
  * **Stderr:** `{"phase":"runtime","code":"LIMIT_EXCEEDED","function":"main","instruction":1,"opcode":"READ_FILE","message":"memory limit exceeded (max 99 bytes)"}`
* **Command:** `bin/howlframe run --allow-caps filesystem --max-memory-bytes 100 audit_programs/test_mem_read_file.howl`
  * **Exit Code:** 0
  * **Output:** `100` (verifying exact 1 byte per byte charge for `read_file`).

#### Program: `audit_programs/test_mem_bytes_to_string.howl`
```lisp
(cli_app
  (let (b (read_file "audit_programs/data_100.txt"))
    (let (s (bytes_to_string b))
      (print (str_len s))
    )
  )
)
```
* **Command:** `bin/howlframe run --allow-caps filesystem --max-memory-bytes 199 audit_programs/test_mem_bytes_to_string.howl`
  * **Exit Code:** 1
  * **Stderr:** `{"phase":"runtime","code":"LIMIT_EXCEEDED","function":"main","instruction":4,"opcode":"CONVERT","message":"memory limit exceeded (max 199 bytes)"}` (verifying `bytes_to_string` charges the string text, totaling 200 bytes with `read_file`).

#### Program: `audit_programs/test_mem_exec.howl`
```lisp
(cli_app
  (let (out (exec "/usr/lib/cargo/bin/coreutils/printf" "%s" "12345678901234567890123456789012345678901234567890"))
    (print (list_len out))
  )
)
```
* **Command:** `bin/howlframe run --allow-caps 'process:exec=/usr/lib/cargo/bin/coreutils/printf %s *' --max-memory-bytes 49 audit_programs/test_mem_exec.howl`
  * **Exit Code:** 1
  * **Stderr:** `{"phase":"runtime","code":"LIMIT_EXCEEDED","function":"main","instruction":3,"opcode":"EXEC","message":"memory limit exceeded (max 49 bytes)"}`
* **Command:** `bin/howlframe run --allow-caps 'process:exec=/usr/lib/cargo/bin/coreutils/printf %s *' --max-memory-bytes 50 audit_programs/test_mem_exec.howl`
  * **Exit Code:** 0
  * **Output:** `50` (verifying exact 1 byte per byte charge for `exec` output).

#### Invalid Resource Flags
* **Command:** `bin/howlframe run --max-instructions 0 audit_programs/hello.howl`
  * **Exit Code:** 2
  * **Output:** `-max-instructions must be greater than zero; unlimited resource usage is not supported`
* **Command:** `bin/howlframe run --max-memory-bytes 0 audit_programs/hello.howl`
  * **Exit Code:** 2
  * **Output:** `-max-memory-bytes must be greater than zero; unlimited resource usage is not supported`
* **Command:** `bin/howlframe run --max-request-bytes 0 audit_programs/hello.howl`
  * **Exit Code:** 2
  * **Output:** `-max-request-bytes must be greater than zero; unlimited resource usage is not supported`
* **Command:** `bin/howlframe run --listen-host localhost audit_programs/hello.howl`
  * **Exit Code:** 2
  * **Output:** `--listen-host must be an IP literal`
* **Command:** `bin/howlframe run --deadline 0s audit_programs/hello.howl`
  * **Exit Code:** 2
  * **Output:** `-deadline must be greater than zero`

---

### 4.7 Capabilities and Process Authority

#### Program: `audit_programs/test_exec_find.howl`
```lisp
(cli_app
  (print (exec "/usr/bin/find" "."))
)
```
* **Command:** `bin/howlframe run --allow-caps 'process:exec=/usr/bin/find **,filesystem' audit_programs/test_exec_find.howl`
  * **Exit Code:** 0
  * **Output:** Byte array representing directory contents.

#### Program: `audit_programs/test_exec_star.howl` (testing `*` pattern flag rejection)
```lisp
(cli_app
  (print (exec "/usr/bin/find" "-name"))
)
```
* **Command:** `bin/howlframe run --allow-caps 'process:exec=/usr/bin/find *' audit_programs/test_exec_star.howl`
  * **Exit Code:** 1
  * **Stderr:** `{"phase":"runtime","code":"CAPABILITY_DENIED","function":"main","instruction":2,"opcode":"EXEC","message":"capability denied: process: exec argv is not in the allow-list"}`
* **Command:** `bin/howlframe run --allow-caps 'process:exec=/usr/bin/find -name' audit_programs/test_exec_star.howl`
  * **Exit Code:** 1
  * **Stderr:** `{"phase":"runtime","code":"IO_ERROR","function":"main","instruction":2,"opcode":"EXEC","message":"exec failed: exit status 1"}` (authorized to dispatch, failed inside child).

#### Program: `audit_programs/test_exec_failure_details.howl`
```lisp
(cli_app
  (try_let (res (exec "/usr/bin/find" "-name"))
    (catch err (print "caught error:" err))
    (print "success:" res)
  )
)
```
* **Command:** `bin/howlframe run --allow-caps 'process:exec=/usr/bin/find -name' audit_programs/test_exec_failure_details.howl`
  * **Exit Code:** 0
  * **Output:**
    ```text
    caught error: {"phase":"runtime","code":"IO_ERROR","function":"main","instruction":2,"opcode":"EXEC","message":"exec failed: exit status 1","exit_code":1,"output":"/usr/bin/find: missing argument to `-name'\n"}
    ```

#### Program: `audit_programs/test_exec_dynamic_argv.howl`
```lisp
(cli_app
  (let (argv (list "-name" "nonexistent*"))
    (print (exec "/usr/bin/find" argv))
  )
)
```
* **Command:** `bin/howlframe run --allow-caps 'process:exec=/usr/bin/find **' audit_programs/test_exec_dynamic_argv.howl`
  * **Exit Code:** 0
  * **Output:** `[]`

#### Program: `audit_programs/test_spawn_agent.howl`
```lisp
(cli_app
  (spawn_agent "Worker" (grants "environment")
    (task "do work"
      (print (env "USER"))
    )
  )
)
```
* **Command:** `bin/howlframe run --allow-caps process,environment audit_programs/test_spawn_agent.howl`
  * **Exit Code:** 0
  * **Output:**
    ```text
    [Swarm VM] Spawning agent "Worker" for task: "do work"
    howlcipher
    [Swarm VM] Agent "Worker" completed task: "do work"
    ```

#### Program: `audit_programs/test_spawn_agent_denied.howl`
```lisp
(cli_app
  (spawn_agent "Worker" (grants "database")
    (task "do work"
      (print "working")
    )
  )
)
```
* **Command:** `bin/howlframe run --allow-caps process audit_programs/test_spawn_agent_denied.howl`
  * **Exit Code:** 1
  * **Stderr:** `{"phase":"runtime","code":"CAPABILITY_DENIED","function":"main","instruction":1,"opcode":"SPAWN_AGENT","message":"capability denied: spawn_agent: child requested a grant the parent does not hold"}`

#### Capability Introspection & Scope Checks
* **Command:** `bin/howlframe run --allow-caps 'process:env=PATH' audit_programs/hello.howl`
  * **Exit Code:** 1
  * **Stderr:** `{"reason":"unknown capability in -allow-caps: \"process:env=PATH\"","line":0,"column":0}`
* **Command:** `bin/howlframe run --allow-caps 'process:env=MY_APP_VAR' audit_programs/hello.howl`
  * **Exit Code:** 0
  * **Output:** `Hello from HowlFrame`
* **Command:** `bin/howlframe run --allow-caps 'network:fetch=127.1:8080' audit_programs/hello.howl`
  * **Exit Code:** 1
  * **Stderr:** `{"reason":"unknown capability in -allow-caps: \"network:fetch=127.1:8080\"","line":0,"column":0}`

#### Program: `audit_programs/test_fetch.howl`
```lisp
(cli_app
  (let (resp (fetch "http://127.0.0.1:9099/test" "GET"))
    (print (list_len resp))
  )
)
```
* **Command:** `bin/howlframe run --allow-caps network audit_programs/test_fetch.howl`
  * **Exit Code:** 1
  * **Stderr:** `{"phase":"runtime","code":"CAPABILITY_DENIED","function":"main","instruction":2,"opcode":"FETCH","message":"capability denied: network: fetch host is not in the allow-list (requires network:fetch=127.0.0.1:9099)"}`

#### Program: `audit_programs/test_granted.howl`
```lisp
(cli_app
  (print (granted "filesystem"))
  (print (granted "filesystem:read=/data"))
)
```
* **Command:** `bin/howlframe run audit_programs/test_granted.howl`
  * **Exit Code:** 0
  * **Output:** `false\nfalse`
* **Command:** `bin/howlframe run --allow-caps filesystem audit_programs/test_granted.howl`
  * **Exit Code:** 0
  * **Output:** `true\ntrue`

---

### 4.8 Native Store and Dictionaries

#### Program: `audit_programs/test_skill_store.howl`
```lisp
(cli_app
  ;; Open persistent or in-memory store
  (store_open kv "memory://test")

  ;; Insert or update record
  (store_put kv "user:100" (dict ("id" "100") ("name" "Alice") ("active" "true")))

  ;; Retrieve record (returns nil if not found)
  (let (rec (store_get kv "user:100"))
    (if (is_nil rec)
      (print "Record not found")
      (print (str_join (list "User: " (map_get rec "name")) ""))
    )
  )

  ;; Delete record
  (store_delete kv "user:100")
)
```
* **Command:** `bin/howlframe run --allow-caps database audit_programs/test_skill_store.howl`
  * **Exit Code:** 0
  * **Output:** `User: Alice`

#### Program: `audit_programs/test_store_file.howl`
```lisp
(cli_app
  (store_open kv "file://data/records.json")
  (store_put kv "user:100" (dict ("id" "100") ("name" "Alice") ("active" "true")))
  (let (rec (store_get kv "user:100"))
    (print (map_get rec "name"))
  )
)
```
* **Command:** `bin/howlframe run --allow-caps database audit_programs/test_store_file.howl`
  * **Exit Code:** 1
  * **Stderr:** `{"phase":"runtime","code":"CAPABILITY_DENIED","function":"main","instruction":1,"opcode":"STORE_OPEN","message":"capability denied: filesystem"}`
* **Command:** `bin/howlframe run --allow-caps database,filesystem audit_programs/test_store_file.howl`
  * **Exit Code:** 0
  * **Output:** `Alice`

#### Program: `audit_programs/test_map_keys.howl`
```lisp
(cli_app
  (let (d (dict ("b" 2) ("a" 1)))
    (let (k (map_keys d))
      (print (str_join k ","))
    )
  )
)
```
* **Command:** `bin/howlframe run audit_programs/test_map_keys.howl` (pure, no capabilities)
  * **Exit Code:** 0
  * **Output:** `a,b`

#### Program: `audit_programs/test_store_keys.howl`
```lisp
(cli_app
  (store_open kv "memory://test")
  (store_put kv "k2" (dict ("v" 2)))
  (store_put kv "k1" (dict ("v" 1)))
  (let (keys (store_keys kv))
    (print (str_join keys ","))
  )
)
```
* **Command:** `bin/howlframe run --allow-caps database audit_programs/test_store_keys.howl`
  * **Exit Code:** 0
  * **Output:** `k1,k2`

---

### 4.9 Numeric Contract and Parser Rules

#### Program: `audit_programs/test_to_float_nan.howl`
```lisp
(cli_app
  (try_let (f1 (to_float "NaN"))
    (catch err1 (print "caught NaN conversion error:" err1))
    (print "parsed NaN:" f1)
  )
  (try_let (f2 (to_float "nan"))
    (catch err2 (print "caught nan conversion error:" err2))
    (print "parsed nan:" f2)
  )
  (try_let (f3 (to_float "Inf"))
    (catch err3 (print "caught Inf conversion error:" err3))
    (print "parsed Inf:" f3)
  )
  (try_let (f4 (to_float "Infinity"))
    (catch err4 (print "caught Infinity conversion error:" err4))
    (print "parsed Infinity:" f4)
  )
  (try_let (f5 (to_float "+Inf"))
    (catch err5 (print "caught +Inf conversion error:" err5))
    (print "parsed +Inf:" f5)
  )
  (try_let (f6 (to_float "-Inf"))
    (catch err6 (print "caught -Inf conversion error:" err6))
    (print "parsed -Inf:" f6)
  )
)
```
* **Command:** `bin/howlframe run audit_programs/test_to_float_nan.howl`
  * **Exit Code:** 0
  * **Output:**
    ```text
    caught NaN conversion error: {"phase":"runtime","code":"CONVERSION_ERROR","function":"main","instruction":0,"opcode":"CONVERT","message":"cannot convert \"NaN\" to float"}
    caught nan conversion error: {"phase":"runtime","code":"CONVERSION_ERROR","function":"main","instruction":0,"opcode":"CONVERT","message":"cannot convert \"nan\" to float"}
    caught Inf conversion error: {"phase":"runtime","code":"CONVERSION_ERROR","function":"main","instruction":0,"opcode":"CONVERT","message":"cannot convert \"Inf\" to float"}
    caught Infinity conversion error: {"phase":"runtime","code":"CONVERSION_ERROR","function":"main","instruction":0,"opcode":"CONVERT","message":"cannot convert \"Infinity\" to float"}
    caught +Inf conversion error: {"phase":"runtime","code":"CONVERSION_ERROR","function":"main","instruction":0,"opcode":"CONVERT","message":"cannot convert \"+Inf\" to float"}
    caught -Inf conversion error: {"phase":"runtime","code":"CONVERSION_ERROR","function":"main","instruction":0,"opcode":"CONVERT","message":"cannot convert \"-Inf\" to float"}
    ```

#### Program: `audit_programs/test_numeric_contract.howl`
```lisp
(cli_app
  (print (= (+ 2 3) 5))
  (print (= (+ 2 3.5) 5.5))
  (print (= (/ 9 2) 4.5))
  (print (= (/ 8 2) 4.0))
  (print (= 2 2.0))
  (print (= 9007199254740993 9007199254740992.0))
  (print (> 9007199254740993 9007199254740992.0))
  (print (= (to_int " 123 ") 123))
  (print (= (to_float " 3.14 ") 3.14))
  (try_let (z (/ 1 0))
    (catch err (print "caught div0:" err))
    (print "div0:" z)
  )
)
```
* **Command:** `bin/howlframe run audit_programs/test_numeric_contract.howl`
  * **Exit Code:** 0
  * **Output:**
    ```text
    true
    true
    true
    true
    true
    false
    true
    true
    true
    caught div0: {"phase":"runtime","code":"RUNTIME_ERROR","function":"main","instruction":0,"message":"division by zero"}
    ```

#### Program: `audit_programs/test_div.howl`
```lisp
(cli_app
  (print (/ 9 2))
)
```
* **Command:** `bin/howlframe run audit_programs/test_div.howl` (Bytecode VM)
  * **Exit Code:** 0
  * **Output:** `4.5`
* **Command:** `bin/howlframe -run audit_programs/test_div.howl` (Interpreter)
  * **Exit Code:** 0
  * **Output:** `4.5`

#### Program: `audit_programs/test_round_trunc.howl`
```lisp
(cli_app
  (print (to_int (+ 2.4 0.5)))
  (print (to_int (+ 2.5 0.5)))
  (print (to_int (- -2.4 0.5)))
  (print (to_int (- -2.5 0.5)))
)
```
* **Command:** `bin/howlframe run audit_programs/test_round_trunc.howl`
  * **Exit Code:** 0
  * **Output:**
    ```text
    2
    3
    -2
    -3
    ```

#### Program: `audit_programs/test_round_abs.howl`
```lisp
(cli_app
  (let (x 2.5)
    (print (round x))
  )
)
```
* **Command:** `bin/howlframe check audit_programs/test_round_abs.howl`
  * **Exit Code:** 0
  * **Output:** `OK: audit_programs/test_round_abs.howl`
* **Command:** `bin/howlframe build audit_programs/test_round_abs.howl`
  * **Exit Code:** 1
  * **Output:**
    ```json
    [{"code":"HFIR_TARGET_INFEASIBLE","severity":"ERROR","message":"construct \"round\" cannot be compiled for the standalone bytecode target: no bytecode lowering exists for it","location":{"filename":"test_round_abs.howl","line":3,"column":13},"contract_version":"v1","target":"bytecode"}]
    ```

#### Program: `audit_programs/test_exp_lit.howl`
```lisp
(cli_app
  (print 1e3)
)
```
* **Command:** `bin/howlframe check audit_programs/test_exp_lit.howl`
  * **Exit Code:** 1
  * **Output:** `{"reason":"undefined reference to \"e3\"","line":2,"column":11}`

#### Program: `audit_programs/test_to_float_exp.howl`
```lisp
(cli_app
  (print (to_float "1e3"))
)
```
* **Command:** `bin/howlframe run audit_programs/test_to_float_exp.howl`
  * **Exit Code:** 0
  * **Output:** `1000`

---

### 4.10 Compiler Lowered/Desugared Constructs

#### Program: `audit_programs/test_str_len.howl`
```lisp
(cli_app
  (let (s "hello")
    (print (str_len s))
  )
)
```
* **Command:** `bin/howlframe build audit_programs/test_str_len.howl -o audit_programs/test_str_len.hfbc && bin/howlframe run audit_programs/test_str_len.hfbc`
  * **Exit Code:** 0
  * **Output:** `5`

#### Program: `audit_programs/test_str_slice.howl`
```lisp
(cli_app
  (let (s "hello world")
    (print (str_slice s 0 5))
  )
)
```
* **Command:** `bin/howlframe build audit_programs/test_str_slice.howl -o audit_programs/test_str_slice.hfbc && bin/howlframe run audit_programs/test_str_slice.hfbc`
  * **Exit Code:** 0
  * **Output:** `hello`

#### Program: `audit_programs/test_str_contains.howl`
```lisp
(cli_app
  (print (str_contains "hello" "e"))
)
```
* **Command:** `bin/howlframe build audit_programs/test_str_contains.howl -o audit_programs/test_str_contains.hfbc && bin/howlframe run audit_programs/test_str_contains.hfbc`
  * **Exit Code:** 0
  * **Output:** `true`

#### Program: `audit_programs/test_starts_ends.howl`
```lisp
(cli_app
  (print (starts_with "hello" "he"))
  (print (ends_with "hello" "lo"))
)
```
* **Command:** `bin/howlframe build audit_programs/test_starts_ends.howl -o audit_programs/test_starts_ends.hfbc && bin/howlframe run audit_programs/test_starts_ends.hfbc`
  * **Exit Code:** 0
  * **Output:** `true\ntrue`

#### Program: `audit_programs/test_reverse_sort.howl`
```lisp
(cli_app
  (let (l (list 3 1 2))
    (do
      (print (reverse l))
      (print (sort l))
    )
  )
)
```
* **Command:** `bin/howlframe build audit_programs/test_reverse_sort.howl -o audit_programs/test_reverse_sort.hfbc && bin/howlframe run audit_programs/test_reverse_sort.hfbc`
  * **Exit Code:** 0
  * **Output:**
    ```text
    [2 1 3]
    [1 2 3]
    ```

#### Program: `audit_programs/test_rec.howl`
```lisp
(cli_app
  (let (r (rec "a" 1))
    (print r)
  )
)
```
* **Command:** `bin/howlframe build audit_programs/test_rec.howl -o audit_programs/test_rec.hfbc && bin/howlframe run audit_programs/test_rec.hfbc`
  * **Exit Code:** 0
  * **Output:** `map[a:1]`

#### Program: `audit_programs/test_when_unless.howl`
```lisp
(cli_app
  (when true
    (print "when works")
  )
  (unless false
    (print "unless works")
  )
)
```
* **Command:** `bin/howlframe build audit_programs/test_when_unless.howl -o audit_programs/test_when_unless.hfbc && bin/howlframe run audit_programs/test_when_unless.hfbc`
  * **Exit Code:** 0
  * **Output:**
    ```text
    when works
    unless works
    ```

#### Program: `audit_programs/test_cond.howl`
```lisp
(cli_app
  (cond
    (false (print "no"))
    (true (print "cond works"))
  )
)
```
* **Command:** `bin/howlframe build audit_programs/test_cond.howl -o audit_programs/test_cond.hfbc && bin/howlframe run audit_programs/test_cond.hfbc`
  * **Exit Code:** 0
  * **Output:** `cond works`

#### Program: `audit_programs/test_inc_dec.howl`
```lisp
(cli_app
  (let (x 10)
    (do
      (inc x)
      (print x)
      (dec x)
      (print x)
    )
  )
)
```
* **Command:** `bin/howlframe build audit_programs/test_inc_dec.howl -o audit_programs/test_inc_dec.hfbc && bin/howlframe run audit_programs/test_inc_dec.hfbc`
  * **Exit Code:** 0
  * **Output:** `11\n10`

#### Program: `audit_programs/test_not.howl`
```lisp
(cli_app
  (print (not true))
  (print (not false))
)
```
* **Command:** `bin/howlframe build audit_programs/test_not.howl -o audit_programs/test_not.hfbc && bin/howlframe run audit_programs/test_not.hfbc`
  * **Exit Code:** 0
  * **Output:** `false\ntrue`

#### Program: `audit_programs/test_mod.howl`
```lisp
(cli_app
  (print (% 10 3))
)
```
* **Command:** `bin/howlframe build audit_programs/test_mod.howl -o audit_programs/test_mod.hfbc && bin/howlframe run audit_programs/test_mod.hfbc`
  * **Exit Code:** 0
  * **Output:** `1`

---

### 4.11 WebAssembly Targets

#### Program: `audit_programs/test_wasm_math.howl`
```lisp
(wasm_app
  (+ 10 32)
)
```
* **Command:** `bin/howlframe -o audit_programs/build audit_programs/test_wasm_math.howl`
  * **Exit Code:** 0
  * **Emitted Artifact:** `audit_programs/build/test_wasm_math.hfbc` (binary bytecode, not `app.wat`)
* **Command:** `bin/howlframe -compile-wasm audit_programs/test_wasm_math.howl`
  * **Exit Code:** 0
  * **Emitted Artifact:** `audit_programs/test_wasm_math.howl.ssa.wat`
* **Command:** `bin/howlframe build audit_programs/test_wasm_math.howl --target wasm`
  * **Exit Code:** 0
  * **Emitted Artifact:** `audit_programs/test_wasm_math.howl.ssa.wat`
* **Command:** `bin/howlframe build audit_programs/test_wasm_math.howl --target wasm -o audit_programs/build/app.wat`
  * **Exit Code:** 0
  * **Emitted Artifact:** `audit_programs/build/app.wat`

---

### 4.12 CLI Interop and Version Metadata

* **Command:** `bin/howlframe version`
  * **Exit Code:** 0
  * **Output:**
    ```text
    HowlFrame 0.1.0
    HFBC format: 2
    ```
* **Command:** `bin/howlframe interop --json`
  * **Exit Code:** 0
  * **Output:**
    ```json
    {
      "schema": "howl.interop/v1",
      "component": "howlframe",
      "version": "0.1.0",
      "hfbc_format": "2",
      "commands": [
        {
          "path": [],
          "flags": [
            "--version",
            "-o",
            "-run",
            "-compile-bc",
            "-compile-hfir-bc",
            "-hfir-adapter-roundtrip",
            "-compile-wasm",
            "-required-caps",
            "-receipt",
            "-run-bc",
            "-allow-caps",
            "-max-instructions",
            "-max-call-depth",
            "-max-memory-bytes",
            "-max-fetch-bytes",
            "-max-exec-output-bytes",
            "-max-request-bytes",
            "-access-log",
            "-timing-log",
            "-listen-host",
            "-deadline",
            "-validate",
            "-mask-plan",
            "-optimization-plan"
          ],
          "json": false,
          "emits": [
            "howlframe.receipt/v0"
          ]
        },
        {
          "path": [
            "inspect"
          ],
          "flags": [
            "--caps"
          ],
          "json": true
        },
        {
          "path": [
            "check"
          ],
          "flags": [
            "--profile"
          ],
          "json": false
        },
        {
          "path": [
            "build"
          ],
          "flags": [
            "-o",
            "--profile",
            "--target"
          ],
          "json": false
        },
        {
          "path": [
            "run"
          ],
          "flags": [
            "--receipt",
            "--target",
            "--allow-caps",
            "--max-instructions",
            "--max-startup-instructions",
            "--max-call-depth",
            "--max-memory-bytes",
            "--max-fetch-bytes",
            "--max-exec-output-bytes",
            "--max-request-bytes",
            "--access-log",
            "--timing-log",
            "--listen-host",
            "--deadline"
          ],
          "json": false,
          "emits": [
            "howlframe.receipt/v0"
          ]
        },
        {
          "path": [
            "version"
          ],
          "flags": [],
          "json": false
        },
        {
          "path": [
            "help"
          ],
          "flags": [],
          "json": false
        },
        {
          "path": [
            "interop"
          ],
          "flags": [
            "--json"
          ],
          "json": true,
          "emits": [
            "howl.interop/v1"
          ]
        }
      ],
      "exit_codes": {
        "0": "ok; under run, the program's own exit status passes through",
        "1": "error: a diagnostic (JSON on stdout for compile errors) or a runtime trap",
        "2": "invalid resource flag (zero, negative, or non-numeric limit)"
      },
      "env": [],
      "consumes": [],
      "description": "HowlFrame language toolchain and capability-bounded VM; runs no other Howl component."
    }
    ```
