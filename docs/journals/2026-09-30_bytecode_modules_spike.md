# Bytecode modules — design spike (#106)

## Decision

Do not build a bytecode-tier module linker yet.

Flat multi-file programs already compile and run on the production bytecode
path. Linking is compile-time AST work from improvements #93 and #95. The
bytecode VM never sees `use`, `export`, or `module`. The remaining rejected
shape is a module that itself contains `(use ...)`. Closing that shape is a
transitive linker. This spike refuses to start one.

No future implementation row is filed here. The row to file later, once the
preconditions below are true, is **Transitive AST module linking**.

## What #95 already covers

#95 stays Done. This spike does not reopen it.

`howlframe.go` runs `parser.ExpandIncludes` and then `ast.ResolveModules`
before `checker.Check`, the HFIR gate, and `bytecode.CompileToBytecode`.
That order is the production `-compile-bc` path (HOWL-CANON-010, Path 1).
`ast.ResolveModules` (`internal/ast/resolver.go`) is one non-recursive pass
over the root's children. It mangles an export to `alias_name`, a private
definition to `alias_private_name`, rewrites `alias/name` call sites through
one global rename map, unwraps `export`, and splices the module body into
the importer.

`internal/construct` classifies `use`, `export`, and `module` as
`CompileTimeOnly`. `internal/bytecode/construct_drift_test.go` lists all
three in `consumedBeforeLowering` and fails if `compileNode` grows a case
for any of them. No bytecode opcode and no VM opcode implements a module.

The closed-artifact property is already tested.
`TestModuleBytecodeRunsWithoutSourceModule` compiles
`tests/module_main.howl` (which uses `tests/module_math.howl`), deletes the
module source, and `-run-bc` still prints `42` and `20`. Private names fail
in `checker.Check` before an artifact is written
(`TestModulePrivateSymbolIsNotReachable`). A missing file fails closed with
a structured diagnostic (`TestModuleMissingImportFailsClosed`). A nested
`use` and a cycle fail closed in `parser.ExpandIncludes` when `depth > 0`,
with the diagnostic "real transitive module linking is not yet supported"
(`TestModuleNestedImportFailsClosed`, `TestModuleCircularImportFailsClosed`).
`TestModuleHFIRProvenanceSurvivesResolution` shows `ast.ResolveModules`
rewrites symbol values and leaves `Filename` alone, so HFIR provenance
still names the module file. HFIR did not gain a linker for that.

`docs/module_system_design.md` §4 records the same boundary. #95 considered
HFIR module linking (its Model B) and VM module opcodes (its Model C) and
rejected both. Model B was the isolated-module HFIR graph linker that #90
does not have. This spike keeps that rejection.

## What the bytecode tier does with a multi-file program

The security-story bytecode tier is the capability-enforced VM reached by
`-compile-bc` and `-run-bc`. `docs/standalone_runtime_blueprint.md` names
that VM the primary standalone runtime. On that path a multi-file program
is one AST by the time lowering starts, then one `.bc.bin` artifact.

`examples/repo_analyst/repo_analyst.howl` is that shape in a real program:
four top-level uses (`discovery`, `classification`, `text_analysis`,
`report`) and no module-to-module `use`. Its README builds it with
`-compile-bc` and runs it with `-run-bc` under `process` and `filesystem`.
The modules export functions. They do not import one another.

`hfir.LowerAST` stamps every node with the entry-file module name passed by
the driver (`filepath.Base` of the importer). After resolution, imported
functions are ordinary nodes in that one graph. `hfir.Module` in
`internal/hfir/storage.go` is a content-addressed set of node ids, import
names, and export names for the HFIR store. Nothing in `LowerAST` or
`LowerToBytecode` reads a HowlFrame `(use ...)` form. `LowerToBytecode`
accepts only a graph and does not call `bytecode.CompileToBytecode`.
`docs/hfir_execution_status.md` keeps module resolution on the AST.
`-compile-hfir-bc` is the experimental Path 2, and it still runs only after
`ast.ResolveModules`.

Reconfirmed on 2026-09-30, with no compiler change, by:

```bash
go test -count=1 -timeout 180s -run 'TestModule|TestModuleConstructsAreCompileTimeOnly|TestScanAcceptsResolvedModuleProgram|TestCompileTimeOnlyConstructsReachingCompileNodeHaveCases' . ./internal/construct ./internal/bytecode
```

The root package, `internal/construct`, and `internal/bytecode` all passed.
This spike adds no linker test. The fixtures above are the evidence.

## Refusals

### Half linker

A half linker is any of the following, and this spike refuses all of them:

- Recursing `ast.ResolveModules` without per-importing-module-qualified
  mangling. Two modules that both `(use "utils.howl" as u)` would emit the
  same `u_name` symbols. #95 deferred that collision on purpose.
- A `compileNode` case for `use` that emits a call to a symbol the artifact
  does not already contain.
- A bytecode import section, or a VM opcode, that loads a second `.bc.bin`
  or reopens a `.howl` file. That breaks the closed-artifact proof and adds
  a second module system beside the AST linker.
- Treating "flatten the import chain" as a bug to patch in place. The
  diagnostic already tells the author to put each `(use ...)` on the
  top-level file. That is the supported contract.

### HFIR module linking

This spike refuses smuggling HFIR module linking into a quick patch.

#90 is still Pending. Its current-reality note says HFIR is a shadow
verification pass: `bytecode.CompileToBytecode` reads the AST, and no
backend consumes HFIR as its lowering source. The 2026-09-29 team backlog
keeps the production path as AST → bytecode and keeps Wasm expansion behind
#90. HOWL-CANON-010 freezes HFIR as the ahead-of-time verifier.

An `Imports` / `Exports` edge on `hfir.Module`, or a `use` resolver inside
`LowerToBytecode`, would be that linker. It would also sit on the
experimental path, so the production VM would still be linked by the AST
pass. Two linkers is the half-linker failure with extra steps.

#90 landing later is the gate for a lowered-HFIR ABI. It is not permission
to add module linking. A lowered ABI can exist while `use` / `export` /
`module` stay `CompileTimeOnly`.

### VM module opcodes

#95 already rejected VM module opcodes. `CompileTimeOnly` plus the drift
test is the lock. This spike does not reopen #95, and it does not add an
opcode.

## ROI against #102–#105 and #108

The near-term rows landed on 2026-09-29. Their published effort
denominators and the diffs that closed them:

| Row | Status | Published effort | Diff that closed it |
| --- | --- | --- | --- |
| #102 HTTP request reads | Done | 4 | 27 files, +1377 / −36 (`e3e8dfe`). Three opcodes, no new capability, no router. |
| #103 absence idiom | Done | 3 | 7 files, +557 / −5 (`990c85a`). Sentinel lock on VM, Go, and JS. |
| #104 chained `map_get` | Done | 3 | 10 files, +561 / −65 (`cb057b7`). Nested reads, no new capability. |
| #105 `html_escape` / `attr_escape` | Done | 4 | 24 files, +1214 / −6 (`1ebfda0`). Two pure opcodes plus handler contract tests. |
| #108 dict/list `TYPE_ERROR` | Done | 4 | 11 files, +868 / −33 (`391d8c7`). Fail-closed guards. No new opcode. |

#108 is Done. It is in the same effort band as this spike's own score
(value 6, effort 4, score 1.50) and it closed a real soft-failure. It is
not a reason to reopen opcode work.

#106's effort 4 is the spike, which this note finishes. A correct
transitive linker is a different size. #93, the AST linker that already
shipped, is published at effort 6. Holding this spike's value at 6 and
using that effort gives `6 × 1 ÷ 6 = 1.0`. That sits under the still-open
rows #107 (1.67, effort 3) and #109 (1.33, effort 3). The value is also
high for the pain that remains: the compiler already accepts a flattened
root `use` list, and Repo Analyst is that program on the bytecode VM. A
value of 4 against effort 6 scores about 0.67.

A half linker sized like #104 (effort 3) would score `6 × 1 ÷ 3 = 2.0` and
would outrank those rows on the formula alone. That score is the trap.
Unqualified recursion collides, and a load-time opcode is a second module
system. The formula is only checkable if the effort denominator is the
real linker, which is #93's class, not a weekend opcode.

`docs/journals/2026-09-29_map_keys.md` called bytecode modules the largest
remaining Frame gap and a compiler redesign, and left it alone. The gaps
behind it (#102–#105, then #108) are now Done. The redesign is still a
redesign, and the flat multi-file bytecode path those rows did not need is
already in tree.

## Before revisiting

File a new improvements row named **Transitive AST module linking** only
when all of the following are true. Do not reuse #95, #106, or #90 as that
row.

1. A named consumer cannot be written as a flat top-level `(use ...)` list.
   `tests/module_main.howl`, `tests/module_math.howl`, and
   `examples/repo_analyst/repo_analyst.howl` do not qualify. They already
   run on `-compile-bc` / `-run-bc`.
2. The row specifies per-importing-module-qualified mangling, a collision
   fixture for two importers of the same module, and a cycle diagnostic
   that is not the `depth > 100` include guard. `use`, `export`, and
   `module` stay `CompileTimeOnly`. The artifact stays closed: `-run-bc`
   must still succeed after the `.howl` sources are deleted.
3. The row adds no VM module opcode and no second module system. #90 may
   still be Pending. If #90 has landed, that ABI still does not authorize
   HFIR import edges for HowlFrame modules. Production lowering stays
   `bytecode.CompileToBytecode` on the resolved AST until a separate
   decision says otherwise.
4. The row's effort denominator stays in #93's class (published effort 6)
   or higher. A score that assumes effort 3 or 4 is a half linker and
   fails this spike's ROI check.

## What this spike does not do

No transitive linker. No bytecode import section. No VM module opcode. No
parser, checker, capability, or runtime change. No new capability for a
pure data operation. No Factory, Board authority, HowlPlane, supervisor
lock, or `owner_direction`. #102, #103, #104, #105, and #108 stay Done and
are not reopened as opcode work. #107 and #109 are not implemented here.
#95 stays Done.

## Acceptance criteria

1. The decision is do not build yet.
2. The note states what #95 already covers: compile-time AST linking,
   `CompileTimeOnly`, and the closed bytecode artifact for a flat `use`.
3. The note refuses a half linker, and names the shapes that count as one.
4. The note refuses an HFIR module-linking patch and leaves #90 as the
   lowered-ABI gate.
5. The ROI comparison uses the published effort denominators and the
   landed diffs for #102–#105, and records #108 Done.
6. Evidence is the existing fixtures and `TestModule*` tests. No new
   linker test was added.
7. The future row is named and not filed. The preconditions for filing it
   are listed above.
