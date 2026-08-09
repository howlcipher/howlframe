# Fix Depth Limit Crash via let Chaining

**Date:** 2026-07-23
**Task:** Fix Bug 9 (Depth Limit Crash via let Chaining)

## Objective
The transpiler was crashing with `AST too deep: exceeded maximum nesting limit of 1000` when given a long script with chained `let` assignments. The nested `{}` blocks were causing deep recursion in `generateStatement`.

## Solution
Modified the `let` handler in `howlframe.go` to iterative unroll contiguous `let` expressions. Instead of passing `depth+1` to a recursively generated body for every `let`, it groups sequential `let` statements into a single, flattened Go scope (`{ ... }`). It also uses a `declaredVars` map to correctly output either `:=` or `=` depending on whether the variable is shadowing an earlier declaration in the same flattened block, thus preventing Go compiler errors.

## Result
`test_let_chain.howl` was tested with 1,500 consecutive `let` bindings and successfully compiled and executed without hitting any stack depth limits. Bug 9 is marked as Done in `bugs.md`.
