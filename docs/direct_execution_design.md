# Direct Execution Design (Improvement #49, Phase 1)

## Vision

Improvement #49, "Direct Neural Bytecode Synthesis," is the last milestone in the V3 "AI-Native Execution" arc: bypass human-readable intermediate languages (Go, JS) entirely, so an AI agent's S-expression eventually lowers straight to something a machine executes with no source-text stage in between. As filed, it carried no concrete design — this document is that design, scoped into phases so a genuinely "weeks"-scale item can ship incrementally, the same way improvement #53 (IR Abstraction) was split into Phase 1/2.

## Why Phase 1 is an interpreter, not a bytecode format

The literal reading of #49 ("an LLM emits target bytecode/IR directly from the S-expression AST") depends on a bytecode *format* existing first — there is nothing today for an LLM to emit. Designing a full instruction set, a serialization format, and a verifier before proving the underlying premise (that skipping text-codegen is viable at all for this language) would be design-first risk with no working checkpoint. Phase 1 instead proves the premise directly: execute the AST `zero.go` already parses, in-process, with no Go/JS text ever generated and no `go build`/`go run`/`node` subprocess ever invoked. If that works and is worth extending, Phase 2 adds a real serialization format (see below) on top of an already-proven execution core; Phase 3 is the actual "LLM writes bytecode" step, once there's a stable format for it to target.

## Phase 1 scope: the `Interpret` function in `zero.go`

A tree-walking interpreter reusing the existing, unchanged front end (`NewLexer`, `NewParser`, `expandIncludes`, `applyPatches`, `applyWithContext`) — only the back end changes. Instead of `generateCode` producing a `(mainCode, testCode string)` pair that gets written to `server.go` and separately compiled, a new `Interpret(ast *Node, args []string) int` walks the same `*Node` tree and executes it immediately, returning a process exit code.

**Implementation note:** this lives in `zero.go` itself, not a separate file. `zero.go` carries `//go:build ignore` specifically so it's excluded from `go build .`/`go vet ./...`/`go test ./...` in the repo root (see Working Protocol point 7) and is instead run standalone via `go run zero.go yourfile.zero`. A separate `interpreter.go` was tried first but broke that invocation two ways: (a) without the same ignore tag, it got swept into every directory-wide `go build`/`go vet`/`go test` and failed to compile (it references `Node`, `reportError`, `binOpKinds`, etc., which only exist in the ignore-tagged `zero.go`); (b) even after tagging it identically, `go run zero.go yourfile.zero` — the one standard invocation used everywhere (README, skill docs, every test script) — only compiles the single file explicitly named, so `zero.go`'s reference to `Interpret` was left undefined unless callers switched to `go run zero.go interpreter.go yourfile.zero` everywhere. That's a breaking change to a universally-documented workflow for zero benefit, so the interpreter's code was merged directly into `zero.go` instead, keeping the single-file invocation exactly as it always was.

Wired into `main()` via a new `-run` boolean flag:
- `./zero -run script.zero [args...]` — interpret and execute immediately, no files written.
- `./zero script.zero` (no `-run`) — unchanged: compile through the Go backend to `server.go`/`server_test.go` as before.
- `-run` combined with an `http_server` or `web_app` root produces a clear JSON error (`{"reason":"-run only supports cli_app in Phase 1",...}`) rather than a silent partial execution.

### Covered node kinds

| Category | Kinds |
| --- | --- |
| Control flow | `let`, `set`, `if`, `while`, `do`, `for`, `return` |
| Functions | `defun`, `call` |
| Output | `print` |
| Expressions | `+ - * / < > <= >= == != = and or` (binop set), `to_int`, `to_float`, `to_string`, `bytes_to_string` |
| Strings | `str_split`, `str_join`, `regex_match` |
| Collections | `list`, `dict`, `append`, `map_set`, `map_delete` |
| Misc | `cli_args`, `sleep`, `env` |

### Explicitly out of scope for Phase 1

`http_server`/`route`/`middleware`, `struct`/`parse_json`/`res`/`res_json`, `db_connect`/`sql_query`, `spawn`, `fetch`, `llm_generate`/`fuzzy_cast`/`assert_semantic`, `try_let`, `patch`, `with_context`, `test`, `match`, `exec`, `read_file`/`write_file`/`mkdir`, `web_app`/JS target. Attempting any of these under `-run` produces a clear "not supported under -run (Phase 1)" JSON error naming the specific node, not a silent no-op or crash. `read_file`/`write_file`/`mkdir` were deliberately deferred rather than included: the Go backend's usage pattern for these depends on `try_let` to catch the `(value, error)` tuple they return, and `try_let` itself is Phase 2 scope — including the I/O primitives without an error-handling primitive to pair them with would mean inventing an ad hoc error convention with no precedent, better decided alongside `try_let` itself.

### Value representation

Since there is no compilation step, there is no Go type system to satisfy — values are dynamically typed (`any`), represented as:
- `int64` for `INT` literals and arithmetic results.
- `string` for `STRING` literals and string ops.
- `bool` for comparison/`and`/`or` results.
- `[]any` for `list`.
- `map[string]any` for `dict`.

This is a deliberate simplification, not an oversight: one of #49's real motivations (see improvement #46's `type_hint` token-overhead finding) is that Go's static typing forces AI-authored Zero code to spend tokens on `type_hint` boilerplate purely to satisfy the *codegen target*, not the logic itself. An interpreter with no compilation step has no such requirement — `defun` parameters and `list`/`dict` elements simply hold whatever value flows into them at call/construction time.

### Two deliberate deviations from the Go backend (not bugs)

1. **`list`/`dict` element types.** The Go backend's `let`-embedded `list`/`dict` codegen (zero.go ~1235–1260) hardcodes `[]string{...}` — every element becomes a Go string literal regardless of source type. The interpreter has no such restriction; elements keep their native type (`int64`, `string`, nested collections).
2. **`for`'s list argument.** The Go backend's `for` codegen reads `node.Children[2].Value` directly, requiring the list to be a bare `SYMBOL` (a variable already holding a `[]string`) — an arbitrary expression there silently produces an empty `.Value` and broken Go. The interpreter evaluates the list position as a full expression, so `(for x (str_split s ",") (print x))` works directly without binding the split result to a variable first. Not a codegen bug fix, just a property this backend doesn't share.

(An earlier draft of this document claimed a third deviation — `if`/`while` condition shape, citing bug #18 as still-open for compound/`and`/`or` conditions. Verified empirically during implementation that bug #18 is actually fixed for both `if` and `while` in the current `zero.go`; the claim was based on a stale detail note in `bugs.md` that contradicted its own ranked-table "Done" status. Removed the false claim here; see the 2026-07-24 journal and the bugs.md groom pass for the correction.)

### Functions and scoping

`defun` bodies are stored in a global function table (name → param list + body `*Node`), matching the Go backend's model where `defun` compiles to an independent top-level function with no closure over caller scope. `call` creates a fresh environment binding only the declared parameters — it does not see the caller's `let` bindings, exactly like the compiled Go equivalent.

`return` is implemented via a Go `panic`/`recover` pair (a `returnSignal` value) so it can unwind out of arbitrarily nested `if`/`while`/`do` blocks, mirroring Go's own `return` semantics without needing every eval function to thread an explicit "did we return" flag.

## Verification approach

For every node kind Phase 1 covers, behavior must be provably identical to the existing Go backend's observable output (stdout), not just "plausible." The verification fixture (`tests/test_interpret_basic.zero`) is run two ways and diffed:

```
./zero -run tests/test_interpret_basic.zero            # Phase 1 interpreter, no files written
./zero tests/test_interpret_basic.zero && go run server.go   # existing Go backend, unchanged
```

Both must produce byte-identical stdout for the fixture to count as verified. This is the same "prove it, don't assert it" discipline #53's byte-diff verification and #44's benchmark both used.

## Phase 2 (done)

- A real serialization format: a flat, versioned instruction list (op code + operands) that Phase 1's AST-walking `eval` lowers to once, rather than re-walking the tree on every run — this is the actual "bytecode" the item's name refers to. Phase 1 deliberately does not build this yet; interpreting the AST directly was sufficient to prove the core premise without committing to an instruction-set design before knowing whether Phase 1 was worth extending.
- Extend node coverage to `http_server`/`route` and the I/O-heavy primitives (`db_connect`, `fetch`, `spawn`) currently excluded.

## Phase 3 (done)

The literal original vision: given a stable Phase 2 bytecode format, have an LLM (via `orchestrator.py`'s existing structured-generation grammar, extended with a bytecode-shaped CFG) emit that format directly instead of `.zero` S-expression text — skipping the lexer/parser stage entirely for agents sophisticated enough to target it directly. Not attempted until Phase 2's format exists and is stable.
