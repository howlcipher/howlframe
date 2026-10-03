# Lowered HFIR ABI: attr_escape on the experimental bytecode path

## Why this slice

`attr_escape` already encodes `&`, `<`, `>`, `"`, and `'` on the interpreter, the AST bytecode VM, Go, and JavaScript. `ATTR_ESCAPE` is an existing opcode and grants nothing. Experimental `-compile-hfir-bc` still rejected it with `HFIR_BYTECODE_UNSUPPORTED`, so the two bytecode compilers were not compared on that construct. `html_escape` already lowers onto `HTML_ESCAPE`. This slice lowers `attr_escape` only, in that same shape. `regex_match` stays rejected. #90 stays Partial.

## What was compared

`LowerAST` gives `(attr_escape text)` one `value` edge. `LowerToBytecode` compiles that text and emits the existing `ATTR_ESCAPE` opcode, with no string operand, the same shape as `bytecode.CompileToBytecode`. A node without that edge returns `HFIR_BYTECODE_UNSUPPORTED` and no `BCProgram`. The model-adapter transport still rejects `attr_escape`. There is no new opcode and no new capability.

`tests/conformance/abi_v1/24_attr_escape.howl` prints the five characters, the combined string, an empty escape inside `[]`, a `let` binding, the taken `if` branch, a `defun` result, one `while` step, and one `for` item. The false branch must not print. `tests/conformance/abi_v1/25_attr_escape_type_error.howl` passes a list and fails with `TYPE_ERROR` and `attr_escape expected string`. Stdout is empty.

| Host | How this case reaches it |
| --- | --- |
| `hfir_bytecode` | `-compile-hfir-bc`, then `-run-bc`. Experimental path. |
| `bytecode` | `-compile-bc`, then `-run-bc`. Production AST bytecode. Canonical result. |
| `interpreter`, `go`, `javascript` | Still the AST. The fixtures use only forms those hosts already run. JavaScript rewrites the root to `web_app`. |

`tools/difftest` runs `attr_escape` and `attr_escape_type_error` from `tests/conformance/lowered_hfir_abi_v1.json`. A stdout, stderr, exit, or error-class mismatch fails the case. The passing case also fails if `untaken` appears. `internal/vm/hfir_equivalence_test.go` round-trips both artifacts and requires the instruction streams to match, including one bare `ATTR_ESCAPE` per call and no `HTML_ESCAPE`. `tests/parity/13_html_escape.howl` calls both escapes. It leaves `hfirRejectedParity` and joins the supported parity set. The encodings match, so that file's instruction streams are compared as well: eight `HTML_ESCAPE` and three `ATTR_ESCAPE`. `tests/parity/07_strings.howl` stays rejected because it calls `regex_match`.

## How to run it

CI runs `go test ./...` (`.github/workflows/ci.yml`), which includes these tests.

Focused commands:

```
go test ./tools/difftest -run 'TestLoweredHFIRABIConformance/attr_escape|TestHFIRBytecodeSupportedParity/13_html_escape'
go test ./internal/vm -run 'TestHFIRBytecodeAttrEscape|TestHFIRBytecodeMixedEscapeParityFile'
go test ./internal/hfir -run 'TestLowerToBytecodeAttrEscape|TestLowerASTAttrEscape'
go test . -run TestProdFlipCriteriaLock
```

## What this does not do

No new opcode. No new capability. Production `-compile-bc` is still `runHFIRGate` and then `bytecode.CompileToBytecode` on the AST. `regex_match` stays outside `LowerToBytecode`. The model-adapter transport still rejects `attr_escape`. No fetch-body implementation. No new Assurance tip-lock. The named lock stays `4d74dbcf9654caa05e0b1d9212b15bc5398359e3`, and this slice leaves it stale. The Decision line stays "Defer the production flip." No Wasm and no module linker. Matching outcomes on these fixtures does not mean `-compile-bc` consumes HFIR. #90 stays Partial.
