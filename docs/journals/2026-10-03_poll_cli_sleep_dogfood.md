# Dogfood: a bounded polling CLI

Tip `73bdb711127315dfc726679ce7f6679b9bc0ec7b`. The compiler was not changed. Production `-compile-bc` was not made the default. #90 was not moved. The Assurance tip-lock `4d74dbcf9654caa05e0b1d9212b15bc5398359e3` was not retaken.

## Program

`examples/poll_cli/poll_cli.howl` is a `cli_app` that records `(time_now)`, then polls up to 3 times. Each attempt is `(sleep 400)` and another `(time_now)`. `time_now` is unix seconds, so three 400ms sleeps are at least 1200ms and the second must change. A change prints `poll ready` plus the attempt count and exits 0. Exhausting the bound prints `poll timeout` plus the count and exits 1.

## What ran

The compiler was `go build -o /tmp/howlframe howlframe.go` from this tree.

| Command | Exit | Result |
| --- | --- | --- |
| `howlframe -validate examples/poll_cli/poll_cli.howl` | 0 | No diagnostic. |
| `howlframe check examples/poll_cli/poll_cli.howl` | 0 | `OK: .../poll_cli.howl`. |
| `howlframe -o /tmp/poll-cli-go examples/poll_cli/poll_cli.howl` | 0 | Wrote `/tmp/poll-cli-go/server.go`. No diagnostic. This is the no-flag Go backend. |
| `go build -o /tmp/poll_cli_go poll_cli_generated_server.go` from the repository root | 1 | The generated file does not compile. See the blocker below. |
| `howlframe -run examples/poll_cli/poll_cli.howl` | 0 | `poll ready 3` in 1.206s. Stderr empty. |
| `howlframe -compile-bc examples/poll_cli/poll_cli.howl -o /tmp/poll_cli.hfbc` | 0 | Artifact written. |
| `howlframe -run-bc /tmp/poll_cli.hfbc` | 0 | `poll ready 2` in 0.805s. Stderr empty. |
| `howlframe build examples/poll_cli/poll_cli.howl -o /tmp/poll-build/poll_cli.hfbc` | 0 | Byte-identical to the `-compile-bc` artifact. |
| `howlframe run /tmp/poll-build/poll_cli.hfbc` | 0 | `poll ready 2` in 0.804s. Stderr empty. |

`-compile-bc` was only read. It was not turned on as the default. `-compile-hfir-bc` was not run.

The production bytecode stream has two `TIME_NOW` instructions, one `SLEEP`, and `PRINT`/`EXIT` on both branches. `SLEEP` has empty operands. The interpreter and both bytecode runs slept: two attempts took about 0.8s and three took about 1.2s.

The timeout line and exit 1 were not observed. With unix-second `time_now` and a 1200ms lower bound, that branch is not reachable for this program.

## Blocker

The default Go backend accepts `(time_now)` and emits no expression and no diagnostic. Both call sites in the generated `server.go` are blank:

```
started :=
if ( > started) {
```

`sleep`, `while`, `set`, `print`, and `exit` are present. `(sleep 400)` is `time.Sleep(time.Duration(400) * time.Millisecond)`. The file still cannot be built, so this host never slept and never printed.

`go build` from the repository root reported:

```
poll_cli.howl:4: syntax error: unexpected = at end of statement
poll_cli.howl:11: syntax error: unexpected >, expected expression
```

`//line` attributes those Go syntax errors to the Howl file. Line 11 is the `(if (> (time_now) started) ...)` call. Line 4 is `(let (limit 3)`, not the empty `(time_now)` binding on line 2. Neither message names `time_now`.

No second sleep, opcode, or language form was added. The program was not rewritten to avoid `time_now`.
