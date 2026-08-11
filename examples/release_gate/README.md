# HowlFrame Release Gate

This demo builds a realistic DevOps/platform-engineering CLI program entirely in HowlFrame. It reads release-readiness signals from a configuration file, parses them using built-in string and dictionary operations, and generates a deterministic deployment recommendation.

## Features Demonstrated

* `try_let` and `catch` for error handling (I/O)
* `read_file`
* `str_split` and loops
* Dictionary creation and mutation (`dict`, `map_set`, `map_get`)
* Deterministic exit codes based on business logic

## Prerequisites

- Go 1.21 or newer
- A built HowlFrame CLI (`go build -o howlframe howlframe.go`)

## Validate and Compile

```bash
./howlframe -validate examples/release_gate/release_gate.howl
./howlframe -compile-bc examples/release_gate/release_gate.howl -o release_gate.hfbc
```

## Run

First, create a sample signals file (e.g., `signals.txt`):

```text
tests=PASS
security=PASS
observability=FAIL
```

Then, run the compiled bytecode. This application requires the `filesystem` capability to read the input file.

```bash
./howlframe -run-bc -allow-caps filesystem release_gate.hfbc signals.txt
```

**Expected output (exit code 0):**
```
RELEASE READINESS
Score: 100
Decision: DEPLOY
```

If you modify the file to have `security=FAIL`, the output will be:

**Expected output:**
```
RELEASE READINESS
Score: 50
Decision: BLOCK
```
