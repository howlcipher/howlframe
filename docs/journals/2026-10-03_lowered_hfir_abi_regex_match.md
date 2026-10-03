# Lowered HFIR ABI: regex_match on the experimental bytecode path

## Why this slice

`regex_match` already matches a pattern against a string on the interpreter, the AST bytecode VM, Go, and JavaScript. `REGEX_MATCH` is an existing opcode. It pops 2, pushes 1, and grants nothing. Experimental `-compile-hfir-bc` still rejected it with `HFIR_BYTECODE_UNSUPPORTED`, so the two bytecode compilers were not compared on that construct. `html_escape` already lowers onto `HTML_ESCAPE`, and `attr_escape` already lowers onto `ATTR_ESCAPE`. This slice lowers `regex_match` only, in that same shape, onto the opcode that already exists. #90 stays Partial.

## What was compared

`LowerAST` gives `(regex_match pattern text)` a `pattern` edge and then a `string` edge. `LowerToBytecode` compiles those edges in that order and emits the existing `REGEX_MATCH` opcode, with no string operand, the same operand order as `bytecode.CompileToBytecode`. The VM pops the string and then the pattern. A node without those edges, or with them swapped, returns `HFIR_BYTECODE_UNSUPPORTED` and no `BCProgram`. The model-adapter transport still rejects `regex_match`. There is no new opcode and no new capability.

`tests/conformance/abi_v1/26_regex_match.howl` prints `true` or `false` for anchored and unanchored patterns, an empty match, a `let` binding, the taken `if` branch, a `defun` result, one `while` step, and one `for` item. The false branch must not print `untaken`. `tests/conformance/abi_v1/27_regex_match_type_error.howl` takes `map_get` of a mixed dict. The checker cannot prove that value is a string. Both bytecode compilers emit `REGEX_MATCH`, and the VM fails with `TYPE_ERROR` and `regex_match expected string pattern and string`. Stdout is empty. A provable non-string, such as a list literal, is rejected by the checker before either compiler runs. The interpreter stringifies the mixed-dict value, generated Go does not compile that dict into `MatchString`, and JavaScript coerces it. That type-error case therefore lists only the two bytecode hosts.

| Host | How this case reaches it |
| --- | --- |
| `hfir_bytecode` | `-compile-hfir-bc`, then `-run-bc`. Experimental path. |
| `bytecode` | `-compile-bc`, then `-run-bc`. Production AST bytecode. Canonical result. |
| `interpreter`, `go`, `javascript` | Still the AST. The passing fixture uses only forms those hosts already run. JavaScript rewrites the root to `web_app`. They are not hosts of the type-error case. |

`tools/difftest` runs `regex_match` and `regex_match_type_error` from `tests/conformance/lowered_hfir_abi_v1.json`. A stdout, stderr, exit, or error-class mismatch fails the case. The passing case also fails if `untaken` appears. `internal/vm/hfir_equivalence_test.go` round-trips both artifacts and requires the instruction streams to match, including one bare `REGEX_MATCH` per call. `tests/parity/07_strings.howl` calls `regex_match` twice. The difftest matched, so that file leaves `hfirRejectedParity` and joins the supported parity set. Its instruction streams are compared as well: two `REGEX_MATCH`. `hfirRejectedParity` is empty. The promote-blocker fence is not.

## How to run it

CI runs `go test ./...` (`.github/workflows/ci.yml`), which includes these tests.

Focused commands:

```
go test ./tools/difftest -run 'TestLoweredHFIRABIConformance/regex_match|TestHFIRBytecodeSupportedParity/07_strings'
go test ./internal/vm -run 'TestHFIRBytecodeRegexMatch|TestHFIRBytecodeStringsParityFile'
go test ./internal/hfir -run 'TestLowerToBytecodeRegexMatch|TestLowerASTRegexMatch'
go test . -run TestProdFlipCriteriaLock
```

## What this does not do

No new opcode. No new capability. Production `-compile-bc` is still `runHFIRGate` and then `bytecode.CompileToBytecode` on the AST. The model-adapter transport still rejects `regex_match`. No fetch-body implementation. `OpFetch` is untouched. No new Assurance tip-lock. The named lock stays `4d74dbcf9654caa05e0b1d9212b15bc5398359e3`, and this slice leaves it stale. The readiness score `docs/journals/2026-10-03_lowered_hfir_prod_flip_score_33cb1325.md` stays a score of `33cb13252bc8e29f4a1bb9d58ce95a82ef6c7358`. This slice does not edit that journal. The Decision line stays "Defer the production flip." No Wasm and no module linker. Matching outcomes on these fixtures does not mean `-compile-bc` consumes HFIR. #90 stays Partial.
