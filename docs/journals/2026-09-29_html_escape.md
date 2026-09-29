# html_escape

## Why this slice

`docs/howlboard_xss_set_html_issue.md` follows task text through string
concatenation into `set_html`, which is `Element.innerHTML`. HowlFrame had no
HTML encoder, so the shortest web program that renders a record is an XSS.
Assurance ranked the escape, data-attribute, and constant-handler contract
tests as release blockers for this surface.

The smallest fix is a pure encoder, not a parser and not a DOM builder.

## Contract

`(html_escape text)` and `(attr_escape text)` encode `&`, `<`, `>`, `"`, and
`'` as `&amp;`, `&lt;`, `&gt;`, `&#34;`, and `&#39;`. That is Go's
`html.EscapeString`. The two names share one helper. `html_escape` is text
context. `attr_escape` is quoted-attribute context. The encoding is the same
five characters, so a quoted attribute does not need a second table.

An empty string stays empty. A non-string fails closed with `TYPE_ERROR` on
the interpreter, the bytecode VM, Go, and JavaScript. Neither opcode declares
a capability. `ForConstruct` stays empty for both names, matching `map_keys`.

`attr_escape` is not a JavaScript encoder. An id placed in a `<script>` body
or an inline handler (`onclick`, `onerror`, and the other `on*` attributes)
is still an interpolation even after `html_escape` or `attr_escape`. The safe
shape puts the escaped id in a data attribute and leaves the handler name a
constant string, for example `data-id="…"` and `onclick="onTask"`.

## What this does not do

No HTML parser. No sanitizer that allows a "safe HTML subset". No new
capability. No `create_element` or `set_text` expansion. `set_html` is still
`innerHTML`. No Board paste-preview work. No #106 modules spike, no #107
capability-honesty docs pass, no #108 dict/list `TYPE_ERROR` sweep, no #109
sort parity. No Factory, HowlPlane, or authority work. The experimental HFIR
bytecode lowerer does not grow an escape node; production bytecode is
`bytecode.CompileToBytecode`.

## Evidence

`internal/htmlescape` locks the five characters, including the combined
string, and the empty string. `ScanHandlers` fails when a concatenated
handler body places a non-constant inside `<script>` or an inline handler,
including when that non-constant is itself an escape call. Constant handler
names stay constants. The same scan reads generated `[]string{…}` and
JavaScript lists, and a `+` chain such as `"<script>" + id`.

`tests/parity/13_html_escape.howl` is the difftest fixture for the
interpreter, the bytecode VM, and Go. `internal/vm/html_escape_test.go`
checks that stdout, including the data-attribute card with `onclick="onTask"`,
and runs the non-string cases as `TYPE_ERROR` under an empty grant.
`internal/backend/gogen/html_escape_test.go` and
`internal/backend/javascript/html_escape_test.go` run the generated programs
and fail if the emitted markup list interpolates a handler.
