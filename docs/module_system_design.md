# Zero Module System Design (Phase 1)

## 1. Inventory of Current Usage

Currently, Zero handles multi-file code reuse through two distinct and unscoped mechanisms:

### `(include "file.zero")`
- **Current Usage**: `examples/routes.zero`, `tests/routes.zero`, and `tests/test_include.zero`.
- **Semantics**: Evaluated purely at the parser level (`internal/parser/parser.go` via `ExpandIncludes`). It performs raw textual AST splicing. The parser reads the target file, parses its AST, and replaces the `(include ...)` node with the children of the parsed AST. If the included AST is wrapped in a `(module ...)` list, the wrapper is stripped and its children are spliced.
- **Issues**: There is zero lexical scoping or namespacing. All included symbols are dumped directly into the global scope, risking collisions. It is a pre-processor macro rather than a module system.

### `(import "pkg")`
- **Current Usage**: `tests/test_schema.zero` uses `(import "github.com/mattn/go-sqlite3")`.
- **Semantics**: Handled in `internal/checker/checker.go` (which simply validates arity) and `internal/backend/gogen/gogen.go` (which extracts the string and appends it to the Go file's `import` block).
- **Issues**: This is a raw, backend-specific passthrough for the Go compiler. It has no semantic meaning in the JS, Wasm, or Bytecode backends. It provides no Zero-level scoping or module boundary.

---

## 2. Options Evaluated

### Option A: The "use / export" Triad (Explicit Scoping)
- **Concept**: A formal module system. Files declare explicitly what they expose, and consumers import them under a specific namespace prefix.
- **Syntax**: 
  - Exposing: `(export (defun foo (x) ...))` or `(export foo)`
  - Consuming: `(use "math.zero" as math)`
  - Calling: `(math/foo 10)`
- **Pros**: Strong encapsulation, clear visibility boundaries, prevents global namespace pollution, maps well to target backend module systems (e.g., JS ES6 modules).
- **Cons**: High implementation effort. Requires significant changes to the checker (symbol environment scoping) and all backend emitters.

### Option B: Named Includes with Namespace Prefixing (Implicit Scoping)
- **Concept**: An evolution of the current `include` AST splicing. The file is still textually spliced, but all top-level symbols from the included file are automatically prefixed.
- **Syntax**: `(include "math.zero" as math)` -> usage: `(math/foo 10)`.
- **Pros**: Lower implementation effort. The parser can simply mangle the AST symbol names during `ExpandIncludes` before the checker even sees them.
- **Cons**: No encapsulation or private visibility. It does not map natively to target language imports (like JS modules or Go packages) and remains a macro-level trick.

### Option C: Shared Zero IR (ZIR) Module Linking
- **Concept**: Defer module linking to the planned Zero IR (ZIR) phase (Improvement #86). Each file compiles to an isolated ZIR graph, and a semantic linker resolves cross-module edges before codegen.
- **Pros**: Architecturally pristine. Highly robust.
- **Cons**: Blocked by massive pending ZIR infrastructure work. Delays providing a working module system to users for too long.

---

## 3. Decision

**Chosen Approach: Option A (Explicit Scoping with `use` and `export`)**

We will implement a true lexical module system with explicit exports and namespace-prefixed consumption. This is the only path that scales for a mature CLI/application language.

### Scoping and Visibility Rules
1. **Private by Default**: Any top-level definition (`defun`, `let`, `struct`) in a file is private to that file unless explicitly wrapped or declared with `export`.
2. **Namespace Prefixing**: `(use "file.zero" as alias)` binds the exported symbols of `file.zero` to the `alias/` prefix. Calling an exported function looks like `(alias/my_func)`.
3. **No Global Pollution**: A `use` declaration does not leak into the global scope.

### Backend Interactions
- **Go Backend**: Translates to Go packages if files are in separate directories, or relies on symbol mangling (e.g., `alias_my_func`) if compiled into a single flat Go `main` package.
- **JS Backend**: Maps cleanly to ES6 modules: `import * as alias from './file.js'`.
- **Wasm/Bytecode**: The compiler/checker will resolve the prefixed symbols to their respective isolated environments and wire up the function pointers/offsets at compile time.
- **Interpreter**: The environment struct will support nested namespace maps.

### Migration Story
To transition existing code safely without breaking the world immediately:
1. **Migrating `(import)`**: The current Go-passthrough `import` will be renamed to `(go_import "pkg")`. This explicitly flags it as a backend-specific FFI directive and frees up the word "import" (or "use") for the Zero-level module system.
2. **Migrating `(include)`**: The existing `(include "file.zero")` will be deprecated. The parser will continue to support it during a transition phase but will emit a compilation warning: `"Warning: (include) is deprecated, please migrate to (use ...)"`. Eventually, `include` will be removed. Existing fixtures like `routes.zero` will be rewritten to use `(export)` and `(use)`.
