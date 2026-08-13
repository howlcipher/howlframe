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

## `howlframe version`
Usage: `howlframe version`

Prints the current version of the HowlFrame CLI and the supported HFBC artifact format version.

---

## Legacy Compatibility Interface
The v0.1 CLI retains full backward compatibility with the original flat flag structure:

* `-validate` : Run lexer, parser, and semantic checker without transpiling.
* `-compile-bc` : Compile AST to bytecode JSON.
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
