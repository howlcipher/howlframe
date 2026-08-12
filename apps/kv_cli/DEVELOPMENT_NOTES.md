# KV CLI Development Notes

## What worked well
- Working with the native store using `store_open`, `store_put`, `store_get`, and `store_delete` is very simple and idiomatic S-expression syntax. No SQL strings are needed.
- `store_get` returns `<nil>` dynamically typed values gracefully for missing keys, making it possible to handle absent data natively.
- Using sequential arguments (like `set foo bar get foo delete foo`) proved that multiple store operations in a single VM process execute deterministically against shared in-memory state.

## Awkward but workable
- Identifying a missing key required `(= (to_string record) "<nil>")`. The language lacks an explicit `nil` literal or `is_nil` intrinsic, which feels slightly leaky when interacting with the Go runtime's formatted representation of `nil`.
- Because `cli_args` in bytecode execution maps exactly to `os.Args` without an array iterator or `list_len`, parsing sequential arguments required manually checking `(!= (cli_args i) "")` and manually incrementing an index variable with deep `do` block nesting.
- Updating a counter requires parsing string `to_int`, incrementing, and converting back `to_string`, because records are untyped dictionaries mostly holding strings. 

## Missing capability
- `list_len` for checking the total number of arguments, which is fundamentally missing from the bytecode execution environment.
- A native `nil` type literal.

## Compiler/runtime bugs discovered
- None.

## Capability findings
- Native store requires `database` capability.
- Without `database`, the program fails fast directly at `STORE_OPEN` with `CAPABILITY_DENIED`.

## Instruction-budget findings
- Processing multiple KV commands inside the same process stayed well within the normal instruction budget. State mutation does not carry significant instruction overhead.

## Developer experience
- Was the language easy to reason about? Yes, the store operations behave like standard dictionaries but decoupled from standard variables.
- What consumed the most effort? Figuring out how to check if a key is absent (`"<nil>"`) without crashing, and deep `let`/`do` nesting for manual index counting.
- What would a developer expect that was missing? Standard `list_len`, standard array iteration, and `nil`.
- Did error messages help? Yes, missing `do` block usage was caught immediately by the compiler (`let expects 2 arguments`).
- Did the VM behave predictably? Yes, state persists exactly as expected across the life of the single process.
