# 2026-08-08 — Improvement #95: Standalone Runtime Module Use/Export

## Starting state

Improvement #95 was Pending, unblocked as of today by bug #43 (module resolution off-by-one, Done) and bug #45 (fail-closed construct registry, Done). `internal/construct` classified `use`, `export`, and `module` as `Unsupported` for the standalone bytecode target, each citing `improvements.md #95`. The original description said to "give `use`/`export` a representation in ZIR, a lowering in the bytecode compiler, and an execution model in the VM" — i.e. runtime module opcodes (Model C in the task brief's framing).

## Architecture investigation

Before writing any opcode, I read `zero.go`'s driver and traced the actual pipeline order:

```
parser.ExpandIncludes(root, ...)      // zero.go:74
ast.ResolveModules(root)              // zero.go:75
ast.ApplyPatches / ApplyWithContext   // zero.go:77-79
checker.Check(root)                   // zero.go:80
runZirGate(...)                       // per mode
bytecode.CompileToBytecode(root)      // -compile-bc only, zero.go:107
```

This ordering is unconditional — it runs before `-validate`, `-run`, `-compile-bc`, and every backend. `ast.ResolveModules` (improvement #93) already performs full compile-time linking: it mangles exported symbols to `alias_name`, private ones to `alias_private_name`, rewrites call sites via a global rename map, and splices the module's statements into the importer's tree in place.

`docs/module_system_design.md` §3 already specifies this exact design: "Wasm/Bytecode: the compiler/checker will resolve the prefixed symbols to their respective isolated environments and wire up the function pointers/offsets at compile time." Building ZIR-level (Model B) or VM-level (Model C) module linking would not just be unnecessary, it would contradict this design doc and require the isolated-module ZIR-graph linker that improvement #90 explicitly does not have yet ("ZIR is not on the execution path at all today — it is a shadow verification pass"). #95 must not smuggle #90's scope in.

**Decision: Model A (compile-time AST-level linking) — already implemented by #93, not a new design.**

## Empirical baseline

I built the binary (`go build -o /tmp/zero95 zero.go`) and drove real fixtures through `-validate`/`-run`/`-compile-bc`/`-run-bc`, using the exact `use`/`module`/`export` syntax already proven by `tests/routes.zero`:

```lisp
; math.zero
(module
  (export (defun add_one (n)
    (type_hint n "int") (type_hint return "int")
    (return (+ n 1))))
  (defun hidden () (type_hint return "int") (return 99)))

; main.zero
(cli_app
  (use "math.zero" as math)
  (print (call math/add_one 41)))
```

Results:
- `-run` and `-compile-bc` + `-run-bc`: both print `42`.
- Moving `math.zero` away after `-compile-bc`, then `-run-bc`: still `42` — bytecode does not reopen source.
- `(call math/hidden)` from the importer: fails closed, `checker.Check` reports `undefined reference to function "math/hidden"` with line/column — `ast.ResolveModules` only ever registers a global rename for *exported* names, so an unexported one accessed via the alias syntax simply never resolves. This is the real namespace contract, not obscurity.
- Missing module: fails closed, `Failed to read used file "nope.zero"` + line/column.
- Multiple exports, two aliases exporting the same name, private helper called internally by an exported function: all already worked correctly.
- Go backend on the same fixture: emits `func math_add_one(n int) int` and calls it correctly — third conformance point agreed.
- Source provenance: `ast.Node.Filename` is set once at parse time (`internal/parser/parser.go`, both for the entry file and for each `use`d file via `filepath.Base(fullPath)`) and `ast.ResolveModules`/`renameInTree` only ever mutate `.Value`, never `.Filename`. `zir.LowerAST` (`internal/zir/lowering.go:36`) copies `Filename: astNode.Filename` straight through, so a spliced-in function still carries its origin file all the way into ZIR with no code change needed.

**Genuine gap found:** nested imports. `main` uses `A`, and `A` itself contains `(use "B" as b)`. `ast.ResolveModules` is a single non-recursive pass over `root.Children`; a `use` nested inside an included module's own body is expanded by `parser.ExpandIncludes` (which does recurse) but never gets its own `resolveModule` pass from `ResolveModules`, so calls into `B` are left unrenamed. Symptom: `undefined reference to function "b/b_fn"` — it did fail, but for a confusing reason rather than a deliberate one. A circular pair (`A` uses `B`, `B` uses `A`) was, before this change, only caught by `parser.ExpandIncludes`'s generic `depth > 100` guard.

## Changes made

1. **`internal/parser/parser.go`** — `ExpandIncludes`'s `use` branch now rejects any `use` found at `depth > 0` (i.e. while expanding a file that was itself pulled in by an enclosing `use`) with a diagnostic naming both the module file and its illegal nested target: `nested module import: "A.zero" uses "B.zero", but only a program's top-level file may declare (use ...); flatten the import chain so the top-level file uses "B.zero" directly (real transitive module linking is not yet supported)`.

   This single check closes both the nested-import requirement and the circular-import requirement: a cycle needs at least one module-to-module `use`, which this now always rejects before the file is even read a second time. Verified: the `A`↔`B` circular fixture now fails in well under a second with this diagnostic, not the depth-100 guard.

2. **`internal/construct/construct.go`** — reclassified `use`, `export`, `module` from `Unsupported` to `CompileTimeOnly`, matching `include`/`patch`/`with_context` (all four are "consumed by an earlier pass before lowering ever sees it", the exact `CompileTimeOnly` definition, and all four are resolved in the same `zero.go:74-79` block). Dropped their `Tracker` field, since `TestOnlyUnsupportedEntriesCarryTrackers` requires trackers only on `Unsupported` entries. Updated each `Note`.

3. **`internal/construct/scan_test.go`** — the old "module import" case asserted a raw, unresolved `(use ...)` scans as an `Unsupported` violation; under the new classification it scans clean (moved into `TestScanAcceptsStructuralListsThatAreNotConstructs` with an explanatory comment). Added `TestScanAcceptsResolvedModuleProgram`, which runs the real `parser.ExpandIncludes` + `ast.ResolveModules` pipeline against a temp-file fixture and asserts `construct.Scan` finds nothing — proving the invariant on a genuinely resolved program, not just a hand-built one.

4. **`internal/construct/construct_test.go`** — removed `use`/`export`/`module` from `TestUnsupportedConstructsWithOwnersCiteThem`'s expectation map; added `TestModuleConstructsAreCompileTimeOnly` to lock the new classification in.

5. **`internal/bytecode/construct_drift_test.go`** — `TestCompileTimeOnlyConstructsReachingCompileNodeHaveCases` maintains a `consumedBeforeLowering` allow-list for `CompileTimeOnly` entries that legitimately have no `compileNode` case (previously `patch`/`with_context`/`include`). Added `use`/`export`/`module` to it.

6. **`tests/module_math.zero` + `tests/module_main.zero`** — new checked-in fixtures using the exact `tests/routes.zero`-proven syntax: a module with two exports (`add_one`, `double`) and a private helper (`bump`) called internally by `add_one`, and a `cli_app` importer calling both exports. Flat in `tests/` (not a subdirectory — `TestZirGateAcceptsAllExistingFixtures`, `TestCompileBcCorpusPartitionMatchesRegistry`, and `tools/difftest` all glob `tests/*.zero` non-recursively). `module_math.zero` is a bare, non-standalone module fragment — same shape as `routes.zero` — so it needs the identical skip-map entry in both `zero_test.go` sweep tests. `module_main.zero` is a real `cli_app` root and needed no exemption anywhere; it is automatically exercised by both sweeps and by `tools/difftest`'s three-way interpreter/Go-backend/bytecode stdout comparison.

7. **`zero_test.go`** — added `module_math.zero` to both existing skip maps (mirroring `routes.zero`'s comment). Added seven new tests using scratch `t.TempDir()` fixtures (these are negative/edge-case shapes that should not become permanent corpus fixtures): `TestModuleInterpreterAndBytecodeParity`, `TestModuleBytecodeRunsWithoutSourceModule`, `TestModulePrivateSymbolIsNotReachable`, `TestModuleMissingImportFailsClosed`, `TestModuleNestedImportFailsClosed`, `TestModuleCircularImportFailsClosed` (with a 10s watchdog proving it fails via the new diagnostic, not the depth-100 guard), `TestModuleZirProvenanceSurvivesResolution`.

8. **Docs** — `improvements.md` #95 rewritten to describe the shipped architecture and marked Done; its summary-table row updated. `docs/module_system_design.md` gained a short §4 recording that the "resolve at compile time" design is now verified in practice, and documenting the nested-import scope boundary.

## Deliberately deferred

Real transitive/multi-level module linking (a module importing another module) is not implemented. Doing it correctly would require per-importing-module-qualified symbol mangling once resolution recurses, to avoid collisions between two different modules that both, say, `use "utils.zero" as u"` — genuine new linker design work, not a hardening pass on the existing single-pass resolver. Per this task's own instruction, no new backlog/tracker item was opened for it; it is recorded here, in `improvements.md #95`'s body, and in the nested-`use` error message text itself as the explicit, current scope boundary. First-class ZIR module linking (Model B) and VM module opcodes (Model C) were both considered and rejected for the reasons in the Architecture investigation section above.

`internal/checker`'s existing `module`/`export` pass-through handling (`checkGoAppHandler`/`checkWebAppHandler`) was left untouched — it is a pre-#95, pre-93-vintage defensive fallback for shapes `ast.ResolveModules` doesn't unpack, and is out of scope. The deprecated `include` path's own cycle behavior (still only the `depth > 100` guard) was also left untouched — `include` is legacy, out of scope, and does not have the mangling/scoping hazards that motivated the `use` restriction. `tests/routes.zero`'s existing skip-map exemption is untouched.

## Verification

- `gofmt -l` on every changed Go file: clean.
- `go vet .`: clean.
- `go test ./internal/parser ./internal/ast ./internal/checker ./internal/construct ./internal/zir ./internal/bytecode ./internal/vm`: all pass.
- `go test ./...`: all pass (`go test .` including the seven new `TestModule*` tests and both corpus sweeps with the new fixtures: `ok zero 35.920s`).
- `go build -o /tmp/zero95-final zero.go`: succeeds (never a bare `go build .` in the repo root, per the Working Protocol).
- Manual subprocess proof (also covered by `TestModuleBytecodeRunsWithoutSourceModule`): compiled `tests/module_main.zero` to bytecode in a scratch dir, deleted the copied `module_math.zero`, ran `-run-bc`, got `42\n20\n`.
- Benchmark gate: this change touches neither `defun`, `type_hint`, `read_file`, `str_split`, nor `test`'s transpiled output for any existing authored Zero source. `ExpandIncludes`'s new check only adds a *rejection* path for a shape that was already mis-linked (never previously a valid, passing program), and the `construct.go` reclassification is a registry annotation with no lowering change. The full existing `tests/*.zero` corpus continues to pass both sweep tests unchanged (proof: `TestZirGateAcceptsAllExistingFixtures` and `TestCompileBcCorpusPartitionMatchesRegistry` both green with no fixture besides the two new ones added). No benchmark re-run needed.
- `git status --short` / `git diff --check`: reviewed before finishing (see final commit for the exact file list).

## Result

Improvement #95 is Done. A real multi-file `cli_app` compiles with `-compile-bc` and runs with `-run-bc` with no runtime dependency on the imported `.zero` source; exported symbols resolve through aliases; private symbols are not reachable through the alias and fail closed with a structured diagnostic; multiple modules and aliases do not collide; missing modules fail closed with source location; nested/circular imports fail closed by construction with a dedicated diagnostic; `-run` and bytecode execution agree; the construct registry accurately reflects that `use`/`export`/`module` are compile-time-only; ZIR receives coherent, correctly-attributed nodes with no code change; bug #45's fail-closed guarantees and bug #43's fix are both intact.
