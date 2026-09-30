# Lowered HFIR ABI, dogfood: write_file and mkdir grant and denial

## Why this slice

Phase 2e mediates `(write_file)` and `(mkdir)` on the interpreter, the bytecode VM, generated Go, and JavaScript. Those conformance cases compile with production `-compile-bc` only. Experimental `-compile-hfir-bc` still rejected both forms, so the two bytecode compilers were not compared on a filesystem write. This slice puts `write_file` and `mkdir` on the experimental lowerer with the opcodes the AST compiler already emits, then compares the two compilers on grant and denial. One program also nests those effects with `if`, `while`, `for`, and `defun`. #90 stays Partial.

## What was compared

`LowerAST` gives `(write_file path data)` a `path` edge and a `data` edge, and `(mkdir path)` a `path` edge. `LowerToBytecode` emits the existing `WRITE_FILE` and `MKDIR` opcodes, path then data, the same order as `bytecode.CompileToBytecode`. A `write_file` without both edges, or a `mkdir` without `path`, returns `HFIR_BYTECODE_UNSUPPORTED` and no `BCProgram`. The model-adapter transport still rejects both kinds. There is no new opcode and no new capability. The grant name stays `filesystem`.

`tests/conformance/abi_v1/15_write_file_capability.howl` and `16_mkdir_capability.howl` now include `hfir_bytecode` with the hosts Phase 2e already runs. An empty grant is `CAPABILITY_DENIED` before the write or `mkdir`, the denial does not include the path, and the path is not created. The `filesystem` grant writes `phase2e-write-marker` or creates the directory, then prints `phase2e-wrote` or `phase2e-made`.

`tests/conformance/abi_v1/17_nested_fs_write.howl` defines `place` and calls it twice.

* `(call place 1)` runs a `while` whose body is a `for`. That `for` has an `if`/`else`. The else writes `/tmp/howlframe-abi-v1-dogfood-write.txt`, creates `/tmp/howlframe-abi-v1-dogfood-dir`, and runs a `for` over the bound list `names`. The inner `for` has an `if`/`else`. A `for` over an empty list and a `while` whose condition is `false` sit on that same branch. Those three would write or create a path and print `no`.
* After the counting loop, an `if` writes `/tmp/howlframe-abi-v1-dogfood-loop.txt` when the count is positive. `(call place 0)` skips the counting loop and creates `/tmp/howlframe-abi-v1-dogfood-miss`.

The taken lines are `L a 0`, `L b 0`, `kept 2`, `result 2`, `miss`, and `empty 0`. Exit code is 0. Stderr is empty. With no grant, the first reached filesystem effect is `write_file`. Both bytecode compilers deny with `CAPABILITY_DENIED` and `capability denied: filesystem` before any path is created. The denial text does not include the path. The `no` branches are absent from stdout, and their paths are absent from the filesystem, when `filesystem` is granted.

| Host | How this case reaches it |
| --- | --- |
| `hfir_bytecode` | `-compile-hfir-bc`, then `-run-bc`. Experimental path. |
| `bytecode` | `-compile-bc`, then `-run-bc`. Production AST bytecode. Canonical result. |
| `interpreter`, `go`, `javascript` | Still the AST. The fixture uses only forms those hosts already run, so they are included. JavaScript rewrites the root to `web_app`. |

`tools/difftest` runs `write_file_denied`, `write_file_granted`, `mkdir_denied`, `mkdir_granted`, `nested_fs_write_denied`, and `nested_fs_write_granted` from `tests/conformance/lowered_hfir_abi_v1.json`. A stdout, stderr, exit, or error-class mismatch fails the case. The forbidden token fails it too. Denial must leave every dogfood path absent. A grant must leave the two marker files and the two live directories, and must not create the untaken paths.

`internal/vm/hfir_equivalence_test.go` reads the same files, rewrites each absolute path into a temp directory, round-trips both artifacts, and snapshots the filesystem after each compiler. The nested test also checks the control-edge shape: one `defun`, two `call`s, two `while` headers, three `if`/`else` nodes, three `for` headers, five `write_file` nodes, and four `mkdir` nodes. A `for` sits inside a `for` and inside a `while`. `write_file` and `mkdir` sit inside the `defun`. The two snapshots must match, including a denial that creates nothing.

## How to run it

CI runs `go test ./...` (`.github/workflows/ci.yml`), which includes both tests.

Focused commands:

```
go test ./tools/difftest -run 'TestLoweredHFIRABIConformance/nested_fs_write|TestLoweredHFIRABIConformance/write_file|TestLoweredHFIRABIConformance/mkdir'
go test ./internal/vm -run 'TestHFIRBytecodeWriteFileFixture|TestHFIRBytecodeMkdirFixture|TestHFIRBytecodeNestedFsWriteFixture'
```

Manual pair, from the repository root, with `-allow-caps filesystem` for the grant and without it for the denial:

```
go run howlframe.go -compile-bc tests/conformance/abi_v1/17_nested_fs_write.howl -o /tmp/fs-write-ast.hfbc
go run howlframe.go -run-bc -allow-caps filesystem /tmp/fs-write-ast.hfbc
go run howlframe.go -compile-hfir-bc tests/conformance/abi_v1/17_nested_fs_write.howl -o /tmp/fs-write-hfir.hfbc
go run howlframe.go -run-bc -allow-caps filesystem /tmp/fs-write-hfir.hfbc
```

## What this does not do

No new opcode. No new capability. Production `-compile-bc` is still `runHFIRGate` and then `bytecode.CompileToBytecode` on the AST. Go, JavaScript, and the interpreter still consume the AST. The model-adapter transport still rejects `write_file` and `mkdir`. `match` and `try` stay without control edges. No Wasm and no module linker. Matching outcomes on these fixtures does not mean `-compile-bc` consumes HFIR. #90 stays Partial.
