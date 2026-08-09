# HowlFrame Module System Design (Phase 1)

## 1. Inventory of Current Usage

Currently, HowlFrame handles multi-file code reuse through two distinct and unscoped mechanisms:

### `(include "file.howl")`
- **Current Usage**: `examples/routes.howl`, `tests/routes.howl`, and `tests/test_include.howl`.
- **Semantics**: Evaluated purely at the parser level (`internal/parser/parser.go` via `ExpandIncludes`). It performs raw textual AST splicing. The parser reads the target file, parses its AST, and replaces the `(include ...)` node with the children of the parsed AST. If the included AST is wrapped in a `(module ...)` list, the wrapper is stripped and its children are spliced.
- **Issues**: There is zero lexical scoping or namespacing. All included symbols are dumped directly into the global scope, risking collisions. It is a pre-processor macro rather than a module system.

### `(import "pkg")`
- **Current Usage**: `tests/test_schema.howl` uses `(import "github.com/mattn/go-sqlite3")`.
- **Semantics**: Handled in `internal/checker/checker.go` (which simply validates arity) and `internal/backend/gogen/gogen.go` (which extracts the string and appends it to the Go file's `import` block).
- **Issues**: This is a raw, backend-specific passthrough for the Go compiler. It has no semantic meaning in the JS, Wasm, or Bytecode backends. It provides no HowlFrame-level scoping or module boundary.

---

## 2. Options Evaluated

### Option A: The "use / export" Triad (Explicit Scoping)
- **Concept**: A formal module system. Files declare explicitly what they expose, and consumers import them under a specific namespace prefix.
- **Syntax**: 
  - Exposing: `(export (defun foo (x) ...))` or `(export foo)`
  - Consuming: `(use "math.howl" as math)`
  - Calling: `(math/foo 10)`
- **Pros**: Strong encapsulation, clear visibility boundaries, prevents global namespace pollution, maps well to target backend module systems (e.g., JS ES6 modules).
- **Cons**: High implementation effort. Requires significant changes to the checker (symbol environment scoping) and all backend emitters.

### Option B: Named Includes with Namespace Prefixing (Implicit Scoping)
- **Concept**: An evolution of the current `include` AST splicing. The file is still textually spliced, but all top-level symbols from the included file are automatically prefixed.
- **Syntax**: `(include "math.howl" as math)` -> usage: `(math/foo 10)`.
- **Pros**: Lower implementation effort. The parser can simply mangle the AST symbol names during `ExpandIncludes` before the checker even sees them.
- **Cons**: No encapsulation or private visibility. It does not map natively to target language imports (like JS modules or Go packages) and remains a macro-level trick.

### Option C: Shared HowlFrame Intermediate Representation (HFIR) Module Linking
- **Concept**: Defer module linking to the planned HowlFrame Intermediate Representation (HFIR) phase (Improvement #86). Each file compiles to an isolated HFIR graph, and a semantic linker resolves cross-module edges before codegen.
- **Pros**: Architecturally pristine. Highly robust.
- **Cons**: Blocked by massive pending HFIR infrastructure work. Delays providing a working module system to users for too long.

---

## 3. Decision

**Chosen Approach: Option A (Explicit Scoping with `use` and `export`)**

We will implement a true lexical module system with explicit exports and namespace-prefixed consumption. This is the only path that scales for a mature CLI/application language.

### Scoping and Visibility Rules
1. **Private by Default**: Any top-level definition (`defun`, `let`, `struct`) in a file is private to that file unless explicitly wrapped or declared with `export`.
2. **Namespace Prefixing**: `(use "file.howl" as alias)` binds the exported symbols of `file.howl` to the `alias/` prefix. Calling an exported function looks like `(alias/my_func)`.
3. **No Global Pollution**: A `use` declaration does not leak into the global scope.

### Backend Interactions
- **Go Backend**: Translates to Go packages if files are in separate directories, or relies on symbol mangling (e.g., `alias_my_func`) if compiled into a single flat Go `main` package.
- **JS Backend**: Maps cleanly to ES6 modules: `import * as alias from './file.js'`.
- **Wasm/Bytecode**: The compiler/checker will resolve the prefixed symbols to their respective isolated environments and wire up the function pointers/offsets at compile time.
- **Interpreter**: The environment struct will support nested namespace maps.

### Migration Story
To transition existing code safely without breaking the world immediately:
1. **Migrating `(import)`**: The current Go-passthrough `import` will be renamed to `(go_import "pkg")`. This explicitly flags it as a backend-specific FFI directive and frees up the word "import" (or "use") for the HowlFrame-level module system.
2. **Migrating `(include)`**: The existing `(include "file.howl")` will be deprecated. The parser will continue to support it during a transition phase but will emit a compilation warning: `"Warning: (include) is deprecated, please migrate to (use ...)"`. Eventually, `include` will be removed. Existing fixtures like `routes.howl` will be rewritten to use `(export)` and `(use)`.

---

## 4. Standalone Bytecode/VM Target (Phase 2 note, added by improvements.md #95, 2026-08-08)

Section 3's "Wasm/Bytecode: the compiler/checker will resolve the prefixed symbols... and wire up the function pointers/offsets at compile time" is now verified to be exactly what happens: `howlframe.go` runs `parser.ExpandIncludes` then `ast.ResolveModules` on the AST before `checker.Check`, the HFIR gate, and `bytecode.CompileToBytecode` ever run, for every mode. `use`, `export`, and `module` are fully consumed by that point — `internal/construct` classifies all three `CompileTimeOnly`, the same category as `include`, `patch`, and `with_context`. No HFIR node, bytecode opcode, or VM opcode exists or is needed for them.

**Current scope boundary — nested/transitive imports are not supported.** `ast.ResolveModules` is a single non-recursive pass over the top-level program's own children, so it cannot safely resolve a `use` found inside a module that was itself pulled in by another `use` (i.e. `main` uses `A`, and `A` itself uses `B`). Rather than mis-link this silently, `parser.ExpandIncludes` rejects any `use` found while expanding an already-included module with a structured diagnostic naming both files. This also means a circular module dependency cannot be constructed at all — a cycle requires at least one module-to-module `use`, which this boundary always rejects. Real transitive module linking (per-importing-module-qualified symbol mangling to avoid collisions once resolution recurses) is deliberately deferred; see `docs/journals/2026-08-08_improvement_95_standalone_modules.md`.
