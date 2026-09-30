# Lowered HFIR ABI: html_escape on the experimental bytecode path

## Why this slice

`html_escape` already encodes `&`, `<`, `>`, `"`, and `'` on the interpreter, the AST bytecode VM, Go, and JavaScript. `HTML_ESCAPE` is an existing opcode and grants nothing. Experimental `-compile-hfir-bc` still rejected it with `HFIR_BYTECODE_UNSUPPORTED`, so the two bytecode compilers were not compared on that construct. This slice lowers `html_escape` only. `attr_escape` and `regex_match` stay rejected. #90 stays Partial.

## What was compared

`LowerAST` gives `(html_escape text)` one `value` edge. `LowerToBytecode` compiles that text and emits the existing `HTML_ESCAPE` opcode, with no string operand, the same shape as `bytecode.CompileToBytecode`. A node without that edge returns `HFIR_BYTECODE_UNSUPPORTED` and no `BCProgram`. The model-adapter transport still rejects `html_escape`. There is no new opcode and no new capability.

`tests/conformance/abi_v1/22_html_escape.howl` prints the five characters, the combined string, an empty escape inside `[]`, a `let` binding, the taken `if` branch, a `defun` result, one `while` step, and one `for` item. The false branch must not print. `tests/conformance/abi_v1/23_html_escape_type_error.howl` passes a list and fails with `TYPE_ERROR` and `html_escape expected string`. Stdout is empty.

| Host | How this case reaches it |
| --- | --- |
| `hfir_bytecode` | `-compile-hfir-bc`, then `-run-bc`. Experimental path. |
| `bytecode` | `-compile-bc`, then `-run-bc`. Production AST bytecode. Canonical result. |
| `interpreter`, `go`, `javascript` | Still the AST. The fixtures use only forms those hosts already run. JavaScript rewrites the root to `web_app`. |

`tools/difftest` runs `html_escape` and `html_escape_type_error` from `tests/conformance/lowered_hfir_abi_v1.json`. A stdout, stderr, exit, or error-class mismatch fails the case. The passing case also fails if `untaken` appears. `internal/vm/hfir_equivalence_test.go` round-trips both artifacts and requires the instruction streams to match, including one bare `HTML_ESCAPE` per call and no `ATTR_ESCAPE`. `tests/parity/13_html_escape.howl` stays rejected because it also calls `attr_escape`.

## How to run it

CI runs `go test ./...` (`.github/workflows/ci.yml`), which includes these tests.

Focused commands:

```
go test ./tools/difftest -run 'TestLoweredHFIRABIConformance/html_escape'
go test ./internal/vm -run 'TestHFIRBytecodeHTMLEscape'
go test ./internal/hfir -run 'TestLowerToBytecodeHTMLEscape|TestLowerASTHTMLEscape'
```

## What this does not do

No new opcode. No new capability. Production `-compile-bc` is still `runHFIRGate` and then `bytecode.CompileToBytecode` on the AST. `attr_escape` and `regex_match` stay outside `LowerToBytecode`. The model-adapter transport still rejects `html_escape`. No Wasm and no module linker. Matching outcomes on these fixtures does not mean `-compile-bc` consumes HFIR. #90 stays Partial.
