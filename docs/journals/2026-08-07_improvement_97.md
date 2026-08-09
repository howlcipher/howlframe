# Improvement #97: Standalone Runtime CLI Semantics: Phase 1 (stdin, stderr, explicit exit codes)

## Objective
Implement core standalone CLI primitives:
- `read_line`: read a line from stdin.
- `stderr`: write to standard error.
- `exit`: exit the process with an explicit status code without a VM internal error.

## Implementation Steps
1. **Opcodes and Compilation**: Added `OpReadLine`, `OpStderr`, and `OpExit` to `internal/bytecode/opcode.go`. Registered them in `internal/bytecode/bytecode.go`.
2. **Type Checking**: Registered `read_line`, `stderr`, and `exit` in `internal/checker/types.go` and verified arity and typing rules.
3. **VM Context Expansion**: Expanded `BCVM` and `Interpreter` structures in `internal/vm/vm.go` to accept `io.Reader` (In), `io.Writer` (Out), and `io.Writer` (ErrOut). Modified `OpPrint` and `eval` to use `vm.Out` instead of `os.Stdout`. Added `bufio.Reader` logic for `read_line`.
4. **Exit Semantics**: Introduced `VmExit` to gracefully panic out of the evaluation loops. Recover blocks in `RunBytecode` and `Interpret` capture `VmExit` and return its code explicitly, enabling the calling CLI to forward the integer code to `os.Exit()`.
5. **CLI Integration**: Updated `howlframe.go` and `cmd/howlframe/main.go` to pass `os.Stdin`, `os.Stdout`, and `os.Stderr` properly to the interpret and bytecode runners.
6. **Tests**: Added comprehensive test cases in `internal/vm/cli_test.go` to verify correct `stdin` consumption, `stderr` redirection, and propagation of arbitrary exit codes like `exit 7`.

## Summary
The VM now accurately models native Unix I/O and process exit capabilities without conflating successful semantic exit states with compiler panics. Phase 1 standalone runtime is implemented.
