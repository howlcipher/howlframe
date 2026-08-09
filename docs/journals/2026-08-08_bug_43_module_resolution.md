# Bug #43 - Module Resolution Drops an Included Module's First Statement

## Date
2026-08-08

## Description
Bug #43 reported that `(use "routes.howl" as routes)` failed to resolve an imported function: `tests/test_include.howl` failed `checker.Check` with `undefined reference to function "getGreeting"`, where `getGreeting` is exported by `tests/routes.howl`.

The symptom was misleading. It looked like selective export resolution — one exported function missing while the same module's `(route "/greet" ...)` worked fine — and the tracker's fix sketch accordingly pointed at `ast.ResolveModules` and improvement #93's module system. Neither was at fault.

## Root Cause
An off-by-one slice in `ExpandIncludes` (`internal/parser/parser.go`), caused by an asymmetry between two different shapes that are both spelled `(module ...)`:

- A **source** module, as written in a `.howl` file, is *unnamed*: `(module stmt...)`. Its statements begin at `Children[1]`.
- The **synthetic** module `ExpandIncludes` builds when rewriting `(use "mod.howl" as m)` is *named*: `(module "m" stmt...)`. Its statements begin at `Children[2]`.

`Children[2:]` is correct for the synthetic form, and every downstream consumer uses exactly that — `ast.ResolveModules` (`internal/ast/resolver.go`), `gogen.flattenModules`, and `checker.resolveNamespaces`. But `ExpandIncludes` was applying the *same* `Children[2:]` slice when copying statements *out of* the unnamed source module:

```go
moduleNode.Children = append(moduleNode.Children, includedAst.Children[2:]...)
```

That silently discarded the first statement of every included module. In `tests/routes.howl` the first statement is the whole `(export (defun getGreeting ...))` block, so `getGreeting` never entered the importing file's scope, while the second statement `(route "/greet" ...)` survived intact — producing a failure that looked export-specific but was purely positional.

The identical defect existed on the deprecated `(include ...)` path.

## Fix
`Children[2:]` → `Children[1:]` on both expansion paths in `internal/parser/parser.go`: the `use` path (line 87) and the `include` path (line 117). Two characters. No change to the module system, the checker, or `ast.ResolveModules`.

## Tests
Three cases added to `internal/parser/parser_test.go`:

- `TestExpandIncludesRetainsModuleStatements` — first statement retained when it *is* an `export`.
- `TestExpandIncludesRetainsModuleFirstStatementNonExport` — first statement retained when it is **not** an export, proving the bug is positional rather than export-specific.
- `TestExpandIncludesIncludeDeprecated` — same guarantee on the deprecated `include` path, which splices statements into the parent list rather than wrapping them.

All three were confirmed to **fail against the reverted one-line change** and pass with it, so they are genuine regression tests rather than restatements of current behavior:

```
--- FAIL: TestExpandIncludesRetainsModuleStatements
    expected module statements [export defun], got [defun]
--- FAIL: TestExpandIncludesRetainsModuleFirstStatementNonExport
    expected module statements [defun export], got [export]
--- FAIL: TestExpandIncludesIncludeDeprecated
    expected children [list defun export], got [list export]
```

## Verification
- `./howlframe -validate tests/test_include.howl` and `./howlframe -compile-bc tests/test_include.howl` both exit 0.
- Transpiling emits `func routes_getGreeting(name string) string`, both the module's `/greet` route and the importer's `/health` route, and a call site correctly mangled to `routes_getGreeting(...)`. The generated `server.go` compiles against the repo module.
- `go build`, `go vet ./...`, `go test ./...`, and `gofmt -l .` are all clean.
- `tools/difftest`: 16 passed / 26 skipped / 0 failed — identical to the pre-change run.

## Exemptions Removed
`tests/test_include.howl` was excluded from `TestHFIRGateAcceptsAllExistingFixtures` and `TestCompileBcCorpusPartitionMatchesRegistry` as a documented pre-existing failure. Both exclusions are gone; the fixture is now covered unexempted.

`tests/routes.howl` remains skipped in both, for its own unrelated reason — it is an include-only fragment with no standalone root, valid only when pulled in by an importer. The comments in `howlframe_test.go` that attributed its skip to bug #43 were stale and have been corrected.

## Fixed in Passing
`TestCompileBcCorpusPartitionMatchesRegistry` ran `construct.Scan` against a **raw** parse, while the real `-compile-bc` path scans a fully expanded AST (`howlframe.go:74-77` runs `ExpandIncludes` and `ResolveModules` before `runHFIRGate` at line 106). While `test_include.howl` was exempt this mismatch was unreachable; removing the exemption exposed it, since the test would have seen a `use` head that the compiler never sees. The test now expands includes and resolves modules before scanning, restoring parity with the production pipeline.

## Tracker Status
Bug #43 marked Done (2026-08-08).

Improvement #95 (standalone module support) remains **Pending** and is not advanced by this work: `module`, `use`, and `export` are still classified `Unsupported` for the bytecode target in `internal/construct`, unchanged. This fix is purely AST-level frontend resolution. Bug #42 untouched.
