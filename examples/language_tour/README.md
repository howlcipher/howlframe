# HowlFrame Language Tour

This demo proves that ordinary HowlFrame code can represent business logic clearly using its S-expression syntax. It implements a simple "release readiness" CLI that evaluates signals and returns a score and a decision.

## Features Demonstrated

* `cli_app` root node
* Modules (`use` and `export`)
* Typed functions (`defun`, `type_hints`)
* Function calls and return values (`call`, `return`)
* Variables and mutation (`let`, `set`)
* Control flow (`if`, `do`)
* String and numeric operations (`str_join`, `+`, `>=`, `to_string`)
* CLI argument parsing (`cli_args`)
* Standard streams and exit codes (`print`, `stderr`, `exit`)

## Prerequisites

- Go 1.21 or newer (to build the compiler/runner)
- A built HowlFrame CLI (`go build -o howlframe howlframe.go`)

## Validate

```bash
./howlframe -validate examples/language_tour/language_tour.howl
```

## Compile

This compiles the application down to HowlFrame bytecode:

```bash
./howlframe -compile-bc examples/language_tour/language_tour.howl -o language_tour.hfbc
```

## Run

This requires no special capabilities since it does not touch the filesystem, network, or external processes.

```bash
./howlframe -run-bc language_tour.hfbc my-project PASS PASS PASS
```

**Expected output:**
```
RELEASE READINESS
Project: my-project
Score:   100
Decision: PASS
```

**Failure case:**
```bash
./howlframe -run-bc language_tour.hfbc my-project PASS FAIL PASS
```

**Expected output (exit code 1):**
```
RELEASE READINESS
Project: my-project
Score:   60
Decision: FAIL
```
