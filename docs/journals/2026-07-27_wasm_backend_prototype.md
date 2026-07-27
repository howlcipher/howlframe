# WebAssembly Backend Prototype (#54)

**Date:** 2026-07-27
**Status:** In progress

## Selection

#54 is the highest-ranked eligible Pending item: score `0.50` (`7×0.5÷7`).
All remaining Pending items are marked below the `0.5` ROI floor. A stopped,
four-day-old `agy` process shares the repository directory but has no open
repository files; `git fetch origin` confirmed `HEAD` matches `origin/main`.

## Re-evaluation

The original target remains valuable as a proof that the shared IR can drive a
third, portable code generator. The current IR covers 23 cross-backend node
kinds, but it does not lower `let`, `defun`, or `call`; without these, a full
Zero-to-Wasm backend is not technically honest. The available environment has
no `wat2wasm`, `wasm-tools`, Wasmtime, or Wasmer. Node is present but cannot
parse WAT on its own.

**Prototype boundary:** add a `wasm_app` root that emits a standards-compliant
`.wat` module for the portable numeric/control-flow subset that can be lowered
directly from the AST/shared IR. It must reject unsupported nodes with a clear
compiler error, export a zero-argument `main` function returning `i32`, and
produce a module that can be validated and instantiated whenever a WAT
toolchain is available. Do not claim direct bytecode generation or support for
backend-native I/O, HTTP, LLM, collection, string, or function primitives.

## Delegate record

Pending: request an `agy` non-Claude model to draft the smallest coherent
implementation and tests. Review the on-disk diff before accepting it.

## Verification plan

1. Unit-test generated WAT shape and unsupported-node diagnostics.
2. Build the transpiler and run the full Go test suite.
3. Run the existing Go fixture compile loop to guard the prior backends.
4. If a WAT validator can be installed or located without changing the repo,
   compile and instantiate the generated fixture; otherwise record the missing
   external validation tool accurately.

## Next step

Obtain and review the delegate draft, then implement the bounded Wasm backend
and its tests.
