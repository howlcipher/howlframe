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
| `literal` | exactly one argument equal to `literal` |
| `*` | exactly one argument, any value |
| `**` | (last token only) zero or more remaining arguments |

With no pattern, the program may run only with zero arguments. Examples:

```
-allow-caps 'process:exec=/usr/bin/find **,filesystem'
-allow-caps 'process:exec=/usr/bin/printf %s *'
```

Rules, enforced identically on the interpreter (`-run`), `-run-bc` (both
`-compile-bc` and `-compile-hfir-bc` artifacts), and generated Go/JavaScript
(`HOWLFRAME_ALLOW_CAPS`):

* No shell is ever involved; argv is passed verbatim.
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
  exit error.
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
  `SPAWN_AGENT` instruction (`spawn_agent: child requested "<grant>", which the
  parent does not hold`) before any child instruction runs, and the receipt
  records a denied `SPAWN_AGENT` effect. Inside an agent body this is isolated
  like any other child failure.
* The grant list is carried in `SPAWN_AGENT`'s `string_operand_2`
  (newline-separated). No opcode was added; older artifacts without it run
  their children with no grants.
* `spawn_agent` remains bytecode-only: the interpreter and the experimental
  HFIR lowering refuse it, and generated Go/JavaScript reject the `(grants ...)`
  form with `UNSUPPORTED_ON_TARGET`.
* `spawn` (an in-agent background task) keeps its agent's grants.
