# HowlFrame Dogfooding Report

This engineering report evaluates building three showcase applications—`language_tour`, `capability_lab`, and `release_gate`—using the current HowlFrame language and standalone bytecode runtime.

## What could be built entirely with current HowlFrame

All three showcase applications were built entirely in HowlFrame using the standalone bytecode runtime (`-compile-bc` and `-run-bc`). No fallback to Go or JavaScript generation was required. The applications successfully demonstrate:

* Multi-module application structure (`cli_app`, `module`, `export`, `use`).
* Core control flow and conditionals (`if`, `for`, `do`, `return`).
* Dictionary and string manipulation (`dict`, `map_set`, `map_get`, `str_split`, `str_join`).
* Basic file I/O with error handling (`read_file`, `write_file`, `try_let`, `catch`).
* Deterministic capability denial when attempting unauthorized file access.

## Where the language felt concise

* **Module definition and usage:** The `(module (export (defun ...)))` and `(use "file" as alias)` system is very explicit and easy to reason about.
* **Control flow:** `(if ...)` and `(for ...)` forms are straightforward.
* **Capability boundaries:** The runtime enforcement of capabilities is frictionless from the developer's perspective; the language requires no special syntax, but the runner deterministically intercepts disallowed intents.

## Where the language felt awkward

* **String joining:** `str_join` requires wrapping its arguments in a `(list ...)` form. In a heavily Lisp-inspired language, one might expect `(str_join "a" "b")` or an `(append ...)` equivalent for strings.
* **String splitting and destructuring:** Because `list_get` is available but there is no `list_len` or destructuring assignment, iterating over parts of a string split (like `key=value`) required a manual `for` loop with a counter to unpack the list.
* **JSON parsing / Structs in standalone runtime:** The `-run-bc` environment explicitly omits `struct` (as it has no representation). While `parse_json` exists, its reliance on a type structure means that parsing structured data into a dynamic dictionary without a strict schema requires manual string splitting instead of standard JSON deserialization.

## Unsupported/generalized gaps discovered

* **No `list_len` instruction:** While `list_get` exists, there is no generic `list_len` or `count` instruction available to the bytecode VM.
* **Lack of standalone JSON parsing to dictionaries:** A mechanism to parse JSON directly into a dynamic `dict` without requiring a `struct` definition would greatly simplify file processing tasks.

## Runtime required

All applications compile to `.hfbc` files and run in the **HowlFrame VM**. The bytecode target provides exactly the environment needed to prove the language's security model (capabilities) and deterministic execution.

## Backlog improvements (Suggestions)

While no compiler features were added during this task to keep the focus on dogfooding, the following generalized deficiencies were noted:

1. **Implement `list_len`:** A basic instruction to query the length of a list.
2. **Dynamic JSON parsing:** Allow `parse_json` to deserialize into a generic `dict` if a `struct` type is not provided.
3. **Variadic `str_join` or `str_concat`:** Simplify combining multiple strings without explicitly allocating a list.
