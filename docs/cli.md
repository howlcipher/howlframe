# Command Line Interface (CLI)

HowlFrame v0.1 provides a clean, subcommand-based CLI for the core compilation and execution workflows.

## `howlframe check`
Usage: `howlframe check <source.howl>`

Parses, type-checks, and verifies the `.howl` source file without emitting an artifact. It ensures that the code follows the syntax and structural rules. It will check the syntax, resolve modules, apply type-checking, and run the target-independent HFIR verification gate.

If successful, it prints `OK: <source.howl>` and exits with status 0. Otherwise, it prints structured diagnostics and exits with a non-zero status.

## `howlframe build`
Usage: `howlframe build <source.howl> [-o <output.hfbc>]`

Compiles a `.howl` source file to a standalone HowlFrame bytecode artifact (`.hfbc`). By default, it outputs to a file with the same basename and the `.hfbc` extension.

## `howlframe run`
Usage: `howlframe run [options] <artifact.hfbc> [arguments...]`

Executes a compiled HowlFrame bytecode artifact.

**Important**: Capabilities are denied by default. You must explicitly grant capabilities to the runtime.

### Options
* `--allow-caps` : Comma-separated capabilities to allow (e.g., `network,filesystem,process,environment,database`). Instructions requiring an unlisted capability are denied and will cause the VM to panic.
* `--max-instructions` : A finite instruction ceiling (default `100000`). Once the ceiling is reached, execution halts to prevent infinite loops and runaway resource consumption.
* `--max-call-depth` : Positive CALL recursion and SPAWN_AGENT nesting ceiling (default `1000`).
* `--max-memory-bytes` : Positive cumulative allocation charge ceiling (default `67108864`, 64 MiB). Charges approximate storage; they are not a measured live heap limit and are never reclaimed.
* `--max-fetch-bytes` : Positive fetch response body ceiling (default `10485760`, 10 MiB).
* `--max-exec-output-bytes` : Positive combined stdout/stderr ceiling for exec (default `10485760`, 10 MiB).
* `--deadline` : Optional positive duration such as `5s` or `100ms`; absent means no wall-clock deadline. Explicit `0s`, negative durations, and invalid durations are rejected. Cancels bytecode fetch, exec, sleep, and model requests and checks between instructions.

These limits also apply to legacy `-run-bc`. The new flags apply to `run` targets `bytecode`/`bc`; AST interpreter memory and deadline behavior is unchanged. Exceeded ceilings produce structured `LIMIT_EXCEEDED`. Blocking `read_line` and HTTP serving are not cancelled by the deadline; print output remains uncapped.


## `howlframe version`
Usage: `howlframe version`

Prints the current version of the HowlFrame CLI and the supported HFBC artifact format version.

---

## Legacy Compatibility Interface
The v0.1 CLI retains full backward compatibility with the original flat flag structure:

* `-validate` : Run lexer, parser, semantic checker, and AOT HFIR verifier without transpiling.
* `-compile-bc` : Compile AST to bytecode JSON (production bytecode compiler, gated by AOT HFIR semantic verifier; HOWL-CANON-010). Emission stays `bytecode.CompileToBytecode`. The promote, defer, and kill checklist is `docs/reference/lowered_hfir_prod_flip_criteria.md`. #90 stays Partial.
* `-compile-hfir-bc` : Compile directly from semantic HFIR to bytecode JSON (experimental research direct lowering pathway; HOWL-CANON-010). This flag stays the dogfood path until the Owner authorizes a flip PR.
* `-run-bc` : Run bytecode from JSON file.
* `-compile-wasm` : Compile typed SSA/CFG to WebAssembly Text.
* `-run` : Interpret and execute a script directly.
* `-mask-plan` : Print the deterministic constrained-decoding mask plan.
* `-optimization-plan` : Print the deterministic compile-time optimization plan.

These flags are not part of the primary v0.1 onboarding path but remain fully supported for existing integrations.

## JavaScript `web_app` generation

`howlframe build` deliberately produces only standalone HFBC artifacts. A
`web_app` therefore cannot be built with that subcommand: bytecode rejects
the JavaScript-only root before writing an artifact. The supported v0.1
generation route is the compatibility source interface:

```bash
howlframe -o build frontend.howl
```

For a `web_app` root, this writes `build/app.js` and, when present,
`build/app.test.js`. This remains a compatibility backend workflow, not an
HFBC build or a browser runtime provided by `howlframe run`.
