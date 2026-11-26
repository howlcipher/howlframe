# 2026-11-26: terminal_app line editing, paste safety, colour and scrolling (improvements.md #119)

## Consumer evidence

HowlPlane's pure HowlFrame TUI (howlcipher/howlplane ADR 0010,
`integrations/howlframe/tui/tui_app.howl`) runs on `terminal_app`. On
2026-11-26 its owner chose six improvements. Four of them need the runtime,
because the runtime owns the terminal and a program never writes terminal
codes:

| Need | Observed on `0d74602` |
| --- | --- |
| paste-safe run confirmation | a paste arrives as keystrokes, so a pasted `y` and line break answer the confirmation by themselves, and the program cannot tell a pasted line from a typed one |
| line editing and history | only Backspace and Ctrl-U; arrow keys are dropped, so a typo early in an objective means retyping it |
| colour | none, so failures and outcomes look like any other line |
| scrolling | a view taller than the screen loses its oldest lines, with no way back |

## Change

* `internal/vm/terminal_input.go` (new):
  * key decoding: CSI and SS3 sequences, with modifiers ignored and unknown sequences dropped whole;
  * the line editor and its in-memory history;
  * bracketed-paste reading;
  * the pure layout helpers `inputView` and `scrollWindow`;
  * the palette (`termStyles`, `styled`).
* `internal/vm/terminal_app.go`:
  * Raw mode turns on bracketed paste (`ESC[?2004h`) and turns it off on every exit path, along with the alternate screen.
  * Input goes through the editor, and every `line` event carries `pasted`.
  * View entries may be `{"text", "style"}`.
  * The paint windows a tall view with indicators and keeps a scrolled view in place when the frame grows.
* `howlframe.go`: the runner flag `--color auto|always|never`. Any other value exits with status 2. The interop descriptor lists the flag.

## Decisions

* **A paste never submits; its line breaks become spaces.**
  * Considered: delivering each pasted line as its own `line` event, as the old keystroke path did.
  * Pro: a multi-line paste keeps its structure.
  * Con: one paste can answer a confirmation and then send the next command. That is the hazard this item removes.
  * Spaces keep a pasted objective on one line, which is what the TUI sends anyway.
* **`pasted` marks the whole line.** A line counts as pasted if any part of it was pasted.
  * Pro: a program can refuse any confirmation that was not typed in full.
  * Con: a typed `y` after an unrelated paste on the same line is refused too. The operator clears the line with Ctrl-U and types it.
* **Control characters inside a paste are dropped, Ctrl-C included,** so a pasted 0x03 cannot interrupt a run.
  * Known limit: typing the start marker `Esc [ 2 0 0 ~` by hand enters paste mode until the end marker arrives.
  * A SIGINT sent from outside still reaches the program.
* **History lives in memory only and is never written down,** because a line can carry anything, including an objective, a path, or a secret pasted by mistake.
  * Pro: nothing to protect at rest.
  * Con: history does not survive a restart.
* **`--color always` colours line mode too and overrides `NO_COLOR`.** This follows the no-color.org rule that an explicit per-run flag wins. `auto` never colours line mode, so piped output and tests stay plain.
* **Styles are names, not codes.** A program can only pick a name from the palette. The runtime writes the SGR codes and still sanitizes the text, so the injection guarantee of #117 holds.
* **A scrolled view keeps its place.**
  * When a frame adds lines while the operator is scrolled up, the count hidden below grows by the same amount, so the lines on screen stay put.
  * At the bottom, the view follows the newest lines, and Enter returns there.

## Test Impact Assessment

* **Behavior changed:**
  * raw-mode keys: new editing and history keys, and Up is no longer ignored;
  * bracketed paste;
  * the `pasted` field on every `line` event, in raw and line mode;
  * styled view entries;
  * `--color`;
  * scrolling.
  * Replay is unchanged, except that it records styled entries as data.
* **Existing tests, and whether they still express intended behavior:**
  * `TestTerminalAppRawModeInAPTY` used Up as its example of an ignored key. Up now recalls history, so the test uses Insert (`ESC[2~`), which is still unbound. Its other assertions are unchanged.
  * The line-mode tests in `terminal_app_cli_test.go` and the replay tests pass unchanged. They read events by field, so the added `pasted` field does not affect them.
* **Added:**
  * `internal/vm/terminal_input_test.go` unit tests:
    * editor operations and the line bound;
    * paste marking;
    * history order, the draft, repeat skipping and the 200 cap;
    * decoding of every bound key and of unbound ones;
    * paste conversion and its bound;
    * the sideways input view;
    * the scroll window, with clamps and its row budget;
    * the palette, unknown styles and `colourWanted`.
  * pty tests:
    * editing and history, including where the cursor is drawn;
    * bracketed paste: enabled, never submits, marked, kept through history, and disabled on exit;
    * colour under auto, `NO_COLOR`, `TERM=dumb`, `never`, and `always` over `NO_COLOR`, with styled text still sanitized;
    * scrolling: indicators, both directions, clamping, keeping place as the view grows, and Enter returning to the bottom.
  * A line-mode `--color` test: plain under auto, coloured under always, and a bad value refused.
  * A replay test for styled entries.
* **Mutation checks:** removing the scroll anchor fails the scroll test at the growth step. Not enabling bracketed paste fails the paste test.
* **Obsolete or duplicate contracts:** none removed. `dropEscapeSequence` is replaced by `readEscape`, which the key decoding test covers.
* **Tier:** unit tests cover the pure helpers and pty integration tests cover the driver. HowlPlane adopts the features in its own change, with replay tests.

## Verification

* `gofmt -l .` and `go vet ./...` are clean.
* `go test -p 1 ./...` passes 77 packages. `TestFetchSecurity/MulticastDial` and `TestGoFetchSecurity/MulticastDial` fail, as they do on unchanged `0d74602` in this sandbox, which has no IPv6 (`dial tcp [ff02::1]:80: socket: address family not supported by protocol`).
* The terminal tests pass with the race detector on in both the test binary and the runtime it builds (`GOFLAGS=-race`).
* `scripts/ci_acceptance.sh` passes: every accepted-application suite, 318 checks.
