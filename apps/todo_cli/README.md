# Todo CLI

This is a stateful command-line task manager application built entirely in HowlFrame, evaluating the viability of state-driven application logic atop the `store` primitive and Phase 1 runtime fixes.

## Features

Todo CLI supports deterministic, isolated CRUD operations via a command-sequence execution model:

* `add <title>`: Appends a new todo item with an auto-incrementing ID.
* `list`: Prints all stored todos.
* `get <id>`: Fetches and displays a specific todo item.
* `complete <id>`: Modifies the status of a todo from `open` to `done`.
* `delete <id>`: Erases a todo from the store.

## Execution Model

Todo CLI utilizes the `memory://session` underlying native store. The storage persists precisely for the duration of one VM invocation and is atomically discarded when the script exits.

Because of this, users pass their entire desired lifecycle as a sequence of command arguments.

### Example Usage

```bash
# Evaluate a full CRUD lifecycle in a single execution
go run ../../howlframe.go -run-bc -allow-caps database todo_cli.howl.bc.bin add "Fix CI" add "Write Docs" list complete 1 list get 1
```

## Security

This application requires the `database` capability to interface with native storage. It does not require `filesystem`, `network`, `process`, or `environment`. Attempting to substitute the `database` capability with an unrelated one will correctly cause the VM to fail closed.

## Dogfooding Phase 2

This application serves as the primary artifact for HowlFrame Application Dogfooding Phase 2, confirming that the new `list_len` operation, `is_nil` intrinsic, and structured `TYPE_ERROR` behaviors successfully resolve the friction discovered in Phase 1. 

See `DEVELOPMENT_NOTES.md` and `../../docs/application_dogfooding_phase_2.md` for full implementation analysis.
