# Process authority: exec allow-lists and spawn_agent attenuation

HowlFrame is a **capability-governed runtime**, not a sandbox. Grants are
logic-level checks inside the host process (see `SECURITY.md`); run untrusted
programs inside external OS isolation.

## `exec` needs an exact-argv allow-list

A grant of `process` with no allow-list **never** authorizes `exec`. Each
permitted program is a separate grant:

```
process:exec=<absolute program path>[ <pattern>...]
```

| pattern | matches |
| --- | --- |
| `literal` | exactly one argument equal to `literal` (literal flags such as `-n` work) |
| `\literal` | a backslash escapes a token: `\*` matches a literal `*`, `\**` a literal `**` |
| `*` | exactly one argument that is **non-empty and does not start with `-`** |
| `**` | (last token only) zero or more remaining arguments, **including flags** |

`*` refuses flags so a wildcard slot cannot inject `-o`, `--exec`, etc.; grant
any flag as a literal. `**` is an explicit "anything" and accepts flags; use it
only for programs where any argv is acceptable. A non-final `**` is an invalid
grant on every path (`-allow-caps` parse error; generated Go/JS refuse the
exec).

With no pattern, the program may run only with zero arguments. Examples:

```
-allow-caps 'process:exec=/usr/bin/find **,filesystem'
-allow-caps 'process:exec=/usr/bin/printf %s *'
```

Rules, enforced identically on the interpreter (`-run`), `-run-bc` (both
`-compile-bc` and `-compile-hfir-bc` artifacts), and generated Go/JavaScript
(`HOWLFRAME_ALLOW_CAPS`):

* No shell is ever involved; argv is passed verbatim.
* exec children start with an **empty environment** (no `PATH`, `HOME`, ...).
  Each `process:env=NAME` grant passes one host variable through to every exec
  child; nothing else is inherited. `process:env=` is independent of the
  `environment` capability (which governs `(env ...)` reads by the program).
* No PATH lookup: the requested program must be an absolute, clean path.
  `printf`, `./printf` and `/usr/bin/../bin/printf` are refused.
* The program path must be symlink-free (it must equal its own
  `EvalSymlinks` result). Calling a symlink to a granted program, or granting
  the symlink itself, is refused. On usr-merged systems `/bin/sh` is a
  symlink; grant and call the canonical target (for example `/usr/bin/dash`).
* Literal arguments cannot contain spaces or commas (grants are
  comma-separated and patterns space-separated); use `*` for such arguments.
* Denial is `CAPABILITY_DENIED` with a reason (`exec argv is not in the
  allow-list`, `not an absolute path`, `is or traverses a symlink`, ...). The
  reason never echoes the program or its arguments. On `-run-bc` the receipt
  records `{"op":"EXEC","capability":"process","decision":"denied"}` and the
  exit error; an allowed exec records the matched entry in the effect's
  `grant` field.
* Interpreter (`-run`) denials are typed `CAPABILITY_DENIED` errors but write
  no receipt: `--receipt` requires `-run-bc`, and the interpreter has no
  receipt recorder. Use `-run-bc` when an audit trail is required.
* Concurrent replacement of a program file after the check is a TOCTOU
  caveat, as for filesystem grants.

## `spawn_agent` children receive only explicit grants (S9 attenuation)

```
(spawn_agent "Name" (grants "environment" "process:exec=/usr/bin/printf **")
  (task "description" body...))
```

* The default is **no grants**: `(spawn_agent "Name" (task ...))` runs its
  body with an empty grant set, whatever the parent holds.
* Every `(grants ...)` entry must be a string literal naming a valid grant;
  anything else is a compile error.
* At spawn time each entry must be covered by the *spawning agent's* grants:
  coarse names and `process:exec=` entries verbatim; `filesystem:read=` /
  `filesystem:write=` roots contained in a parent root of the same mode (or a
  coarse parent `filesystem`). A child can never hold more than its parent, so
  escalation through nested spawns is impossible.
* Spawning requires the coarse `process` grant; an exec allow-list alone does
  not let an agent spawn. A nested spawner must therefore be handed
  `(grants "process")`.
* A request for more than the parent holds raises `CAPABILITY_DENIED` at the
  `SPAWN_AGENT` instruction (`spawn_agent: child requested a grant the parent
  does not hold`; the requested grant text is never echoed) before any child
  instruction runs, and the receipt
  records a denied `SPAWN_AGENT` effect. Inside an agent body this is isolated
  like any other child failure.
* The grant list is carried in `SPAWN_AGENT`'s `string_operand_2`
  (newline-separated). No opcode was added; older artifacts without it run
  their children with no grants.
* `spawn_agent` executes its body only on the bytecode VM. The interpreter and
  the experimental HFIR lowering refuse it. Generated Go/JavaScript accept the
  two-argument form but **do not run the task body**: they only print the
  spawning/completed lines. They reject the `(grants ...)` form with
  `UNSUPPORTED_ON_TARGET`.

## `spawn` and HTTP route handlers are attenuated too

* `(spawn (lambda () body))` runs its background body with **no grants**;
  `(spawn (grants "cap" ...) (lambda () body))` hands an explicit subset,
  checked against the spawner like `spawn_agent` (`SPAWN` `string_operand`
  carries the list). An over-request raises `CAPABILITY_DENIED` at `SPAWN`.
  Failures inside a spawned body stay silent, as before.
* `(route "/path" (grants "cap" ...) (lambda (req) body))`: a handler runs with
  `network` (required to answer: `res`, `res_json` and `res_header` are
  network-classified, and `HTTP_ROUTE` already required it) **plus only** its
  `(grants ...)` list (`HTTP_ROUTE` `string_operand_3`). A route without a list
  gets `network` alone, so store, environment, filesystem and process access
  in a handler must be listed. An over-request fails closed when the route is
  registered, before the server listens.
* `(http_server PORT (grants "cap" ...) routes...)`: one `(grants ...)` form
  directly under `http_server` is the default list for every route that does
  not declare its own; a route's own `(grants ...)` (even empty) replaces it.
* Both forms are bytecode-only; the Go checker/backends reject them with
  `UNSUPPORTED_ON_TARGET`.

## Environment pass-through and receipts (#166)

* `process:env=NAME` refuses dynamic-loader variables: any name starting with
  `LD_` (including `LD_PRELOAD`, `LD_LIBRARY_PATH`, `LD_AUDIT`) or `DYLD_`,
  compared case-insensitively. Passing one to an allow-listed program would
  let it load arbitrary code. The grant is rejected at parse time on the
  interpreter and `-run-bc`; generated Go and JavaScript skip such names
  when building the child environment.
* Receipt `grant` strings are copied verbatim from the normalized grant. An
  allowed exec records the matched entry with every pattern literal exactly
  as written (for example `process:exec=/usr/bin/git log --oneline *`).
  Pattern literals are therefore visible to anyone who can read receipts; do
  not put secrets in grant patterns.

