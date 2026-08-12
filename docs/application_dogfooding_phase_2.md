# Application Dogfooding Phase 2

This phase validated whether the core improvements developed in Phase 1 (`list_len`, `is_nil`, structured VM `TYPE_ERROR`) allowed for a recognizable, stateful CRUD-style CLI application to be built.

## Todo CLI
We built `apps/todo_cli`, a command-sequence-driven task manager utilizing the native HowlFrame `memory://session` store. It successfully implements `add`, `list`, `get`, `complete`, and `delete` commands within a strictly enforced capability environment. 

## Did the Phase 1 fixes work?
Yes. 
* **`list_len`**: Made iterating through the raw command-line arguments completely natural and predictable. No string-fallback counters were needed.
* **`is_nil`**: Allowed robust, native handling for absent store records and keys.
* **VM type safety**: Prevented Go-level panics, meaning out-of-bounds evaluation failed predictably.

## Repeated friction
* **`let` nesting (Awkward)**: Just like in Phase 1's Log Analyzer and KV CLI, heavily sequenced procedural logic forces massive rightward drift. The Todo CLI's nested command dispatch resulted in 16+ levels of indentation.
* **Boolean representation (Reasonable)**: The absence of `true` and `false` boolean literals requires strings or dummy conditions, though it remains functionally sound.

## New friction
* **Key Enumeration (Awkward)**: Because there is no native way to retrieve all keys in a store, nor a way to append natively to a store list without retrieving/re-writing it entirely, we were forced to implement a sequential ID counter.

## Known issues that remain tolerable
* Deep `let` nesting is visually unpleasant and ergonomically taxing, but it did not fundamentally block logic or introduce bugs.

## Core defects discovered
No HowlFrame core defects were discovered during this application build! The runtime correctly executed the intended logic on the very first try.

## Application-development assessment
* **CRUD logic**: Reasonable
* **State**: Easy
* **Collections**: Reasonable (Iteration is easy, native arrays/sorting is missing)
* **CLI**: Awkward (Due to `let` nesting depth)
* **Errors**: Easy
* **Modules**: Easy (N/A)
* **Capabilities**: Easy
* **Diagnostics**: Reasonable

## Decision

> **READY FOR TASK API**

### Reasoning
The CRUD logic is fully maintainable, `list_len` resolved collection manipulation problems, and `is_nil` cleanly handles missing states. The core runtime has proven completely capable of executing heavily stateful applications in standalone bytecode isolation. The remaining issues are entirely ergonomic (namely, nested `let` structures and boolean literals), and there are absolutely no blocking runtime defects preventing the construction of larger HTTP/CRUD systems.
