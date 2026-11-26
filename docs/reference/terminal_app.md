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
  * `view`: a list of entries. Each is a line of text, or a `{"text": ..., "style": ...}` dict whose style names the fixed palette (see [Styles](#styles)).
  * `prompt`: the input prompt.
  * `effects`: a list of effect dicts.
* **Failure.** A call that fails, or returns something else, leaves the previous state in place. The runtime then delivers a `handler_failed` event whose `error` is a `howlframe.runtime_error/v1` object.
  * If the `handler_failed` handler fails too, that failure is dropped.
  * If the handler fails on an `interrupt`, the program quits with code 130, so a broken handler cannot trap the operator.

## Events

| `type` | Fields | When |
| --- | --- | --- |
| `start` | `tty`, `cols`, `rows` | first event |
| `line` | `text`, `pasted` | the operator pressed Enter; `pasted` is true when any of the line arrived by paste (see [Paste](#paste)) |
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

* **Raw mode.** When stdin and stdout are a terminal, the runtime takes raw mode and edits the input line itself (improvements.md #119):
  * The runtime switches to the alternate screen, so the operator's scrollback is left intact.
  * Every paint clears the screen, wraps lines to the width with a two-space hanging indent, and keeps the prompt on the last row.
  * The previous mode is restored on quit, on a fatal error and on SIGTERM or SIGHUP.
* **Line mode.** Otherwise the runtime reads whole lines, which is also how tests drive it. It writes a frame only when the view or prompt changed. Every `line` event carries `"pasted": false`.
* **Sanitizing.** The runtime replaces control characters in the view and prompt with `?`, so neither the program nor a child's output can move the cursor or send escape sequences. Only the runtime writes terminal codes.

### Keys

| Key | Action |
| --- | --- |
| printable characters | insert at the cursor |
| Left, Right, Ctrl-B, Ctrl-F | move the cursor |
| Home, End, Ctrl-A, Ctrl-E | go to the start or end of the line |
| Backspace | delete before the cursor |
| Delete | delete under the cursor |
| Ctrl-D | `eof` on an empty line, otherwise delete under the cursor |
| Ctrl-U, Ctrl-K | delete to the start, or to the end, of the line |
| Ctrl-W | delete the word before the cursor |
| Up, Down, Ctrl-P, Ctrl-N | walk the session history |
| PageUp, PageDown | scroll a view taller than the screen |
| Ctrl-L | redraw the screen |
| Enter | send the line as a `line` event |
| Ctrl-C | send an `interrupt` event |

Any other key or escape sequence is dropped whole, so it never reaches the line as text. A line longer than the screen scrolls sideways to keep the cursor in view.

### History

Up and Down recall the lines sent earlier in this session, at most 200, skipping blank lines and repeats of the newest entry. The line being edited is kept and comes back after the newest entry. History lives only in memory and is never written down, because a line can carry anything.

### Paste

Raw mode turns on bracketed paste. A paste is inserted at the cursor as text: line breaks and tabs become spaces and other control characters are dropped, so a paste can never press Enter or any other key. The line is sent only when the operator presses Enter, and its `line` event then carries `"pasted": true`. A line recalled from history keeps the mark it was sent with. A program can use the flag to refuse a pasted answer to a confirmation.

### Scrolling

A view taller than the screen shows its newest lines and a `-- N more lines above: PageUp --` indicator. PageUp and PageDown move by a page, with an indicator for the lines hidden on each side. A new frame keeps a scrolled view in place, so lines added below do not move what the operator is reading, and Enter returns to the bottom.

### Styles

A view entry `{"text": "...", "style": "<name>"}` names one style of a fixed palette:

| Style | Drawn as |
| --- | --- |
| `plain` | default |
| `bold`, `heading` | bold |
| `dim` | dim |
| `info` | cyan |
| `ok` | green |
| `warning` | yellow |
| `error` | red |

Any other style name is plain. Only the runtime turns a style into terminal codes, and styled text is sanitized like any other, so a program never sends an escape of its own. The runner flag `--color` decides whether styles are drawn:

* `auto` (the default) draws them only in raw mode, when `NO_COLOR` is unset or empty and `TERM` is not `dumb`.
* `always` draws them in line mode too, and overrides `NO_COLOR`.
* `never` never draws them.

When styles are not drawn, a styled entry shows as its text alone. A program cannot read or set `--color`.

## Testing with `--replay`

`howlframe run --replay events.jsonl app.howl` runs a `terminal_app` against a recorded event file instead of the terminal. It makes a program's decisions testable deterministically, with no terminal, no children and no grants.

* **Input.** Each non-blank line of the file is one event object with a string `type`. It reaches the handler exactly as written; the runtime adds no `start` event, so begin the file with one if the program expects it.
* **No authority used.** Nothing is painted, so `--color` has no effect, no input is read, and no effect runs: `exec_start`, `exec_signal`, `timer`, `tail` and `append_log` are recorded, not executed. Feed their results back as events (`process_exit`, `timer`, `tail_lines`) to continue the story. Effect kinds are still checked against the closed table, so an unknown kind produces `effect_failed` as it would live.
* **Same rules as live.** Per-event budgets and the state cap apply. A failed handler call keeps the previous state and produces `handler_failed`. A `quit` effect ends the replay with its code, and a `terminate` event stops it with 143.
* **Output.** One `howlframe.terminal_replay/v1` JSON line per handled event, in order, on standard output:

| Field | Meaning |
| --- | --- |
| `schema`, `seq` | `howlframe.terminal_replay/v1`, and a 1-based sequence number |
| `source` | `replay` for an event from the file, `runtime` for one the runtime generated (`handler_failed`, `effect_failed`) |
| `event` | the event as the handler received it |
| `ok`, `error` | whether the handler call succeeded; on failure, the `howlframe.runtime_error/v1` object |
| `state`, `view`, `prompt` | after the event; a failed event leaves them unchanged; the view is recorded as the handler returned it, styled entries included |
| `effects` | the effects the handler returned, as data |
| `quit` | the quit code once the program has quit, otherwise `null` |

`--replay` is runner owned, like `--receipt`: a program cannot read or set it.

## Runner flags

| Flag | Default | Meaning |
| --- | --- | --- |
| `--max-event-instructions` | 100000 | instruction budget of one handler call |
| `--max-event-memory-bytes` | 16 MiB | allocation budget of one handler call |
| `--max-state-bytes` | 1 MiB | encoded state carried between events |
| `--color` | `auto` | when view styles are drawn: `auto`, `always` or `never` (see [Styles](#styles)) |

Zero and negative limits are refused, like every other limit, and any other `--color` value exits with status 2.
