# Log Analyzer Development Notes

## What worked well
- Graceful missing-file behavior and handling capability denials using `try_let` block works very elegantly. Since capability denial triggers a structured panic at runtime inside the VM, the `catch` block catches both normal I/O errors and `CAPABILITY_DENIED` cleanly.
- `regex_match` handles standard pattern checking smoothly and is fast enough to comfortably chew through hundreds of lines.

## Awkward but workable
- `let` expressions require exactly `(var val) body`, so binding multiple variables consecutively creates deep nesting if not strictly organized. Adding a trailing `do` block is required to sequence multiple statements at the tail.
- `read_file` returns raw bytes that are pushed to the bytecode VM as a list of integers, so I had to discover and use `bytes_to_string` before string operations like `str_split` could work.
- Counting the length of a list requires a manual loop counter `line_count` because the language lacks an explicit `list_len` (it is rejected by `-compile-bc` with `HFIR_TARGET_INFEASIBLE` because it lacks a bytecode lowering). 

## Missing capability
- `list_len` for bytecode execution. Workaround: compute list lengths manually via a `for` loop, which costs instruction budget.

## Compiler/runtime bugs discovered
- None directly in the compiler, though confusing `interface {} is []interface {}, not string` panic occurred inside the VM when `str_split` was fed raw bytes without `bytes_to_string`. Proper IR typing or runtime checks in `OpStrSplit` would make this diagnostic better.

## Capability findings
- Requires only `filesystem`.
- When `filesystem` is omitted, the `read_file` call panics with `CAPABILITY_DENIED`. The `catch` block handled it perfectly. No unnecessary capabilities are required.

## Instruction-budget findings
- Processing a 300+ line log file ran well within the default 100,000 instruction limit. The budget works well for lightweight deterministic log analysis.

## Developer experience
- Was the language easy to reason about? Yes, execution order is straightforward.
- What consumed the most effort? Figuring out the data type returned by `read_file` and how to process it as a string, and dealing with `let` nesting.
- What would a developer expect that was missing? A `len` or `list_len` builtin, and a friendlier type-error when passing `[]interface{}` into a string opcode.
- Did error messages help? The compiler's `HFIR_TARGET_INFEASIBLE` message was exceptionally clear about `list_len` not being supported in bytecode.
- Did the VM behave predictably? Yes.
