# HowlFrame Application Dogfooding — Phase 1

## Applications
* **Status API**: A small network service serving HTTP routes, checking environment configuration, and enforcing network capability.
* **Log Analyzer**: A deterministic CLI application processing files, performing string splits, regex matching, and tracking error state gracefully with try/catch logic.
* **KV CLI**: A stateful command-line interface running against HowlFrame's native in-memory bytecode store, proving sequential data persistence within a single process.

## What HowlFrame already does well
- **Capability Enforcement**: The fail-closed bytecode capability model works perfectly. Requesting unauthorized capabilities (like `network` or `filesystem`) results in immediate, structured `CAPABILITY_DENIED` execution panics that are easy to catch via `try_let`.
- **HTTP Server**: The `http_server` routing and `res_json` forms are incredibly simple and effective. It compiles flawlessly into bytecode and handles routing deterministically.
- **State Manipulation**: Native store forms (`store_open`, `store_put`, `store_get`, `store_delete`) behave elegantly without needing an underlying SQL query layer.

## Repeated friction
- **Deep Nesting in let**: Because `let` requires exactly a `(var expr) body` and the `body` only accepts a single AST node, sequentially binding variables or executing multiple side-effects mandates deep, rightward-drifting `do` block nesting. This appeared across all applications.
  - **Status**: Still unresolved
- **Type Friction**: `read_file` returns raw bytes which caused an unexpected `[]interface{}` internal panic when blindly fed into string operations like `str_split`. It required explicit discovery and usage of `bytes_to_string`.
  - **Status**: Addressed after Phase 1 dogfooding (VM now emits structured `TYPE_ERROR`)
- **Missing list_len**: There is no bytecode equivalent for measuring array sizes. Both Log Analyzer and KV CLI had to write manual `while` loops or manual counters in `for` loops to process lists and arguments. 
  - **Status**: Addressed after Phase 1 dogfooding
  - **Evidence**: Log Analyzer + KV CLI + earlier showcase work

## Missing primitives
- **HIGH**: `list_len` in bytecode. Attempting to use `list_len` in `-compile-bc` fails entirely.
  - **Status**: Addressed after Phase 1 dogfooding
- **HIGH**: Explicit `nil` checks. Checking if a store key exists required formatting the response to a string and comparing against `"<nil>"`.
  - **Status**: Addressed after Phase 1 dogfooding (via `is_nil` intrinsic)
- **MEDIUM**: Improved `let` body semantics to accept implicit `do` block execution rather than forcing explicit `(do ...)` wrappers for multiple expressions.
  - **Status**: Still unresolved

## Bugs exposed by applications
- **Type Panic in VM**: While not a strict compiler bug, the bytecode VM `str_split` instruction panics with an internal Go slice conversion error when passed `read_file` bytes instead of a string. An explicit type cast requirement or cleaner VM runtime failure should be implemented.
  - **Status**: Addressed after Phase 1 dogfooding. Fixed evidence-backed unsafe VM operand type assertions to produce structured `TYPE_ERROR` runtime errors.

## Capability model findings
- Minimum-authority execution worked naturally.
- Explicit capability flags (`-allow-caps`) successfully sandboxed `filesystem`, `network`, `environment`, and `database` interactions across the apps.
- When authority was denied, the VM panicked correctly at runtime, failing closed, which proved the boundary is watertight.

## Runtime/backend findings
- All three applications functioned brilliantly in **standalone bytecode**. The `-run-bc` execution path handled HTTP servers, file processing, and native state management natively without relying on the Go code generator.

## Developer experience
- **CLI development**: Reasonable (friction with `cli_args` lacking bounds checking APIs)
- **HTTP development**: Easy
- **file processing**: Reasonable (friction with byte-to-string mapping)
- **collections**: Awkward (missing `list_len` and tricky iteration)
- **strings**: Reasonable
- **state**: Easy (native store is very clean)
- **errors**: Easy (`try_let` gracefully handles errors)
- **modules**: N/A for this phase
- **capabilities**: Easy
- **diagnostics**: Reasonable (compiler errors are very clear, VM panics slightly opaque)

## Suggested next improvements
1. **Bytecode `list_len`**: Add the `list_len` opcode to `-compile-bc` to allow normal array bounds checking. *(Addressed)*
2. **Native `nil` primitive**: Introduce an explicit `nil` literal or `is_nil` intrinsic to handle absent data natively. *(Addressed)*
3. **Ergonomic `let` evaluation**: Allow `let` to accept multiple body expressions (implicit `do`). *(Still unresolved)*
4. **Bytecode type assertions**: Protect string operations in `internal/vm/vm.go` from panicking internally when receiving `[]interface{}`. *(Addressed)*

## Recommended next application
**READY FOR TODO CLI**

HowlFrame successfully built these three initial applications, and the friction around list processing (`list_len`), nil detection (`is_nil`), and VM type-safety has been successfully addressed. A data-heavy program like `apps/todo_cli` can now be built comfortably relying on the new list and nil primitives. The `let` ergonomic issue remains, but is no longer a critical blocker.
