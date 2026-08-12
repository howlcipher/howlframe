# Todo CLI Development Notes

## What worked well
* **State Operations**: `store_put`, `store_get`, and `store_delete` mapped perfectly to CRUD operations for a Todo application.
* **Capabilities**: The `database` capability effectively fenced the state operations. Without it, the application immediately panics on the `store_open` line.
* **Diagnostics**: The newly fixed structured `TYPE_ERROR` cleanly handles wrong argument types, avoiding any raw Go panics.
* **Testing**: Since native storage runs in memory per session, we were able to pass all command-line arguments in one integrated execution (`add Fix CI add Update docs list complete 1...`) and test an entire state lifecycle cleanly and deterministically.

## What was awkward but workable
* **Nested `let` Evaluation**: Because `let` requires exactly a `(var expr) body` where `body` is a single AST node, chaining variable declarations and actions forces extreme rightward indentation via nested `do` blocks. This was already evident in Phase 1 but became highly pronounced with CRUD logic, resulting in 16+ levels of indentation.
  * *Intended behavior*: Define multiple variables and execute sequential statements.
  * *Actual required*: `(let (a 1) (do (let (b 2) (do ...))))`
* **Boolean representation**: There are no boolean literals, so we used string equivalents (`"open"`, `"done"`) and inverted checks (`(if (is_nil record) ...)` with a no-op statement). It was completely workable but less semantically rich than standard language features.

## Missing capabilities
* **Sort / Iterate**: Although `list_len` works on lists, there is no way to natively list all keys in the store or sort them. We worked around this by implementing a deterministic `next_id` counter and sequentially iterating `0..max_id`.

## Bugs discovered
* None. The core functionality used by Todo CLI was solid. The fixes applied after Phase 1 resolved the issues that would have otherwise blocked this app.

## New primitive evaluation
### `list_len`
It materially improved CLI argument iteration. We were able to natively get the bounds of the `cli_args` list and sequentially process it with a simple `while (< i argc)`.

### `is_nil`
It cleanly eliminated the previous string sentinel hack `(= (to_string record) "<nil>")`. The native absence check handled unknown commands and missing records efficiently.

### Structured VM type errors
No internal Go panics leaked. When malformed type structures (or absence of arguments) were evaluated, they produced expected HowlFrame abstraction-level diagnostics.

## State model findings
* **Lifetime**: Memory-bound. Discarded at the end of the session.
* **Determinism**: Fully deterministic ID sequences via a counter key.
* **Missing-key semantics**: Handled correctly by `is_nil`.
* **Update behavior**: Overwriting an existing key natively replaces the record.
* **Deletion behavior**: `store_delete` cleans up the record; subsequent fetches correctly return nil.
* **Persistence limitations**: Since it is `memory://session`, a separate storage backend (like SQLite or JSON files) would be necessary for a real-world CLI, but memory storage proved exactly what was needed for deterministic lifecycle tests.

## Capability findings
Only `database` is required. Attempting to use `filesystem` or `network` is not needed and accurately blocked by the VM if requested in lieu of `database`.

## Instruction-budget findings
* **Normal workload**: Well within the default limits.
* **Stress workload**: Adding 100 Todos in one execution loop completed deterministically within standard instruction bounds (under 1 second).

## Developer experience
* **Data modeling**: Easy, relying on dicts.
* **CRUD logic**: Reasonable, mapped 1-to-1 with store commands.
* **CLI parsing**: Awkward due to `let` nesting, but structurally sound.
* **Collections**: Reasonable thanks to `list_len`.
* **State**: Easy.
* **Errors**: Easy, using explicit string prints and exits.
* **Modules**: Unnecessary for this app size.
* **Capabilities**: Easy and robust.
* **Diagnostics**: Reasonable.
* **Nesting**: **Awkward / Nuisance-level**. The deep right-drift is the primary remaining pain point for readability.
