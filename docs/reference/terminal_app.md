# `terminal_app`: interactive terminal programs

`terminal_app` runs an interactive terminal program, such as an operator console, as plain HowlFrame (improvements.md #117). The runtime owns the terminal, the timers and the child processes. The program supplies one handler, which the runtime calls once per event.

```lisp
(cli_app
  (defun on_event (state event)
    (type_hints (state dict) (event dict) (return dict))
    (return (dict ("state" state)
                  ("view" (list "line one" "line two"))
                  ("prompt" "> ")
                  ("effects" (list)))))
  (exit (terminal_app "on_event" (dict ("count" 0)))))
```

`(terminal_app "handler" initial_state)` names a `defun` with two parameters, `state` and `event`. It returns the code of the `quit` effect that ends the loop. It can run only once per program, because its input reader owns stdin for the rest of the run. It runs on the bytecode VM only; the interpreter, HFIR and the Go, JavaScript and WebAssembly targets do not support it.

## The handler

* **Isolation.** Each call runs in a fresh VM:
  * It has **no grants**: no exec, file, network, environment or store access. Authority is exercised only through effects, which the runtime runs under the program's own grants.
  * It has its **own instruction and memory budget** (`--max-event-instructions`, `--max-event-memory-bytes`). A long session never exhausts a cumulative budget.
  * It cannot read stdin. `print` and `stderr` inside it are discarded, so output cannot corrupt the screen. `exit` is refused; use a `quit` effect instead.
* **Return value.** The handler returns a dict:
  * `state`: carried to the next event as JSON, at most `--max-state-bytes` encoded.
  * `view`: a list of lines.
  * `prompt`: the input prompt.
  * `effects`: a list of effect dicts.
* **Failure.** A call that fails, or returns something else, leaves the previous state in place. The runtime then delivers a `handler_failed` event whose `error` is a `howlframe.runtime_error/v1` object.
  * If the `handler_failed` handler fails too, that failure is dropped.
  * If the handler fails on an `interrupt`, the program quits with code 130, so a broken handler cannot trap the operator.

## Events

| `type` | Fields | When |
| --- | --- | --- |
| `start` | `tty`, `cols`, `rows` | first event |
| `line` | `text` | the operator pressed Enter |
| `interrupt` | none | Ctrl-C (raw mode) or SIGINT |
| `eof` | none | input closed, or Ctrl-D on an empty line |
| `resize` | `cols`, `rows` | the terminal changed size |
| `terminate` | none | SIGTERM or SIGHUP; after this event the program stops (code 143 unless it quits itself) |
| `timer` | `id` | a `timer` effect fired |
| `process_exit` | `id`, `exit_code`, `stdout`, `stderr`, `error` | an `exec_start` child ended; `exit_code` is -1 for a signal, a limit or a start failure, and `error` explains why |
| `tail_lines` | `id`, `path`, `lines`, `offset`, `missing` | answer to `tail` |
| `effect_failed` | `kind`, `id`, `error` | an effect was refused or failed |
| `handler_failed` | `event_type`, `error` | the handler failed |

## Effects (closed table)

| `kind` | Fields | Authority and bounds |
| --- | --- | --- |
| `quit` | `code` | ends the loop; live children are ended, timers stopped |
| `timer` | `id`, `ms` | 0 to 86400000 ms; a second `timer` with the same `id` replaces the first; at most 64 live |
| `exec_start` | `id`, `argv` | authorized exactly like `exec`: an exact-argv `process:exec=` grant, the program's `process:env=` names or a runner `process:exec_inherit_env=` grant; `--max-processes`, `--max-exec-millis` and `--max-exec-output-bytes` apply; stdout and stderr are captured separately |
| `exec_signal` | `id`, `signal` | `interrupt` or `terminate` go to the child's whole process group; `kill` ends the tree |
| `tail` | `id`, `path`, `offset` | `filesystem:read`; reads at most 1 MiB from `offset` and returns only whole lines and the offset after them |
| `append_log` | `path`, `line` | `filesystem:write`; appends one line (newlines become spaces, at most 64 KiB), creating the file with mode 0600 |

At most 64 effects run per event. Any other `kind` fails with `UNKNOWN_EFFECT`. Receipts record each effect's capability decision as `EXEC`, `READ_FILE` or `WRITE_FILE`.

## The terminal

* **Raw mode.** When stdin and stdout are a terminal, the runtime takes raw mode and edits the input line itself:
  * Supported keys: printable characters, Backspace, Ctrl-U, Enter, Ctrl-C and Ctrl-D. Escape sequences such as arrow keys are ignored.
  * The runtime switches to the alternate screen, so the operator's scrollback is left intact.
  * Every paint clears the screen, wraps lines to the width with a two-space hanging indent, and keeps the prompt on the last row.
  * The previous mode is restored on quit, on a fatal error and on SIGTERM or SIGHUP.
* **Line mode.** Otherwise the runtime reads whole lines, which is also how tests drive it. It writes a frame only when the view or prompt changed.
* **Sanitizing.** The runtime replaces control characters in the view and prompt with `?`, so neither the program nor a child's output can move the cursor or send escape sequences. No colors are emitted, so `NO_COLOR` holds trivially.

## Testing with `--replay`

`howlframe run --replay events.jsonl app.howl` runs a `terminal_app` against a recorded event file instead of the terminal. It makes a program's decisions testable deterministically, with no terminal, no children and no grants.

* **Input.** Each non-blank line of the file is one event object with a string `type`. It reaches the handler exactly as written; the runtime adds no `start` event, so begin the file with one if the program expects it.
* **No authority used.** Nothing is painted, no input is read, and no effect runs: `exec_start`, `exec_signal`, `timer`, `tail` and `append_log` are recorded, not executed. Feed their results back as events (`process_exit`, `timer`, `tail_lines`) to continue the story. Effect kinds are still checked against the closed table, so an unknown kind produces `effect_failed` as it would live.
* **Same rules as live.** Per-event budgets and the state cap apply. A failed handler call keeps the previous state and produces `handler_failed`. A `quit` effect ends the replay with its code, and a `terminate` event stops it with 143.
* **Output.** One `howlframe.terminal_replay/v1` JSON line per handled event, in order, on standard output:

| Field | Meaning |
| --- | --- |
| `schema`, `seq` | `howlframe.terminal_replay/v1`, and a 1-based sequence number |
| `source` | `replay` for an event from the file, `runtime` for one the runtime generated (`handler_failed`, `effect_failed`) |
| `event` | the event as the handler received it |
| `ok`, `error` | whether the handler call succeeded; on failure, the `howlframe.runtime_error/v1` object |
| `state`, `view`, `prompt` | after the event; a failed event leaves them unchanged |
| `effects` | the effects the handler returned, as data |
| `quit` | the quit code once the program has quit, otherwise `null` |

`--replay` is runner owned, like `--receipt`: a program cannot read or set it.

## Runner flags

| Flag | Default | Meaning |
| --- | --- | --- |
| `--max-event-instructions` | 100000 | instruction budget of one handler call |
| `--max-event-memory-bytes` | 16 MiB | allocation budget of one handler call |
| `--max-state-bytes` | 1 MiB | encoded state carried between events |

Zero and negative values are refused, like every other limit.
