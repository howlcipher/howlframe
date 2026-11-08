# AI-DX1 measurement preregistration

This protocol was recorded before implementing the understanding interface. It does not report an efficiency experiment or authorize paid calls. The correctness fixtures are deterministic; live matched-agent trials remain deferred pending a verified no-payment route.

## Fixed comparison

Baseline source revision: `44821dc8a08f8942889a864ed25ac9a2e186383d`. Treatment is the eventual merged AI-DX1 revision, recorded in the task evidence. Both conditions use the same executable toolchain dependencies, model version, instructions and model/tool budgets; record their exact versions before running. The five matched tasks are:

1. Find the accepted argument counts for `print`, including whether parameter types are known.
2. Find bytecode `read_file` support and distinguish required authority from grants actually supplied.
3. Check the pure fixture and state whether this proves runtime success.
4. Diagnose the arity fixture and report a stable code and source location.
5. Determine bytecode feasibility of the unsupported fixture without silently discarding its test.

Fixtures use UTF-8 and one trailing newline. Freeze these exact bytes:

### pure

```howl
(cli_app (print (+ 20 22)))
```

SHA-256: `e2e063ebba358a2a80886fac6c353b58ea380370d769eeb91e64d12fb1b4b84c`.

### authority

```howl
(cli_app (print (read_file "input.txt")))
```

SHA-256: `c1f3ab3959720036de945d6908f87baea7eb25833cf41518f4688038db629ae8`.

### arity

```howl
(cli_app (append (list 1)))
```

SHA-256: `3219c76fd18e2098542b5a3de52295a75ac45f7d90856d620d4bfd6b9f893ace`.

### unsupported

```howl
(cli_app (test "sample" (assert true)))
```

SHA-256: `b22a25004def14eba55c8fd8286b59b0333aca3c8107d23c55724d4290261d4c`.

## Execution and accounting

Use equal fixed budgets chosen and recorded before the first live trial. Randomize paired order with a recorded seed; use the same starting filesystem and dependencies, with separate disposable copies. Hold behavioral checks out of model context. Include predefined interruptions after edits, during verification and after evidence recording, and retain all interrupted or failed attempts. Initially run each task in a clean session; any maintenance/resumption extension must be preregistered separately.

Record total input and output context, tool output context, command count, retries, repair payload and verification costs, elapsed time, correctness and unsupported or unknown claims. Snapshot and recovery overhead are recorded as zero only when no snapshot/recovery mechanism is used, never inferred as savings. Report every paired task, dispersion, failed tasks and aggregate costs. Measure AI-DX1 separately from M6 and later session/repair milestones. The roadmap's percentage reductions remain experimental targets; correctness and resumption reliability must not regress.

Deterministic acceptance verifies output consistency, empty actual grants, target isolation, normal compiler rejection, stable diagnostic codes/locations and executable examples. It cannot establish reduced model context, invalid usage or total time.
