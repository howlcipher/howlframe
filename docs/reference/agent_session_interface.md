# Local session evidence (AI-DX2 initial slice)

`agent snapshot`, `agent context` and `agent status --verified` emit exactly one
JSON document with schema `howlframe.agent.session/v1`. Exit 0 requires authenticated,
current evidence; any failure exits 1 with `ok:false`, `verified:false` and a coded
diagnostic. The existing understanding commands retain `howlframe.agent/v1`.

```sh
howlframe agent snapshot --task example --root /absolute/project \
  --bundle /absolute/evidence/new-bundle --state-dir /absolute/private/trust \
  --source main.howl --claim 'Implementing the feature' --next 'Review behavior'
howlframe agent status --verified --task example --root /absolute/project \
  --bundle /absolute/evidence/new-bundle --state-dir /absolute/private/trust
howlframe agent context --task example --root /absolute/project \
  --bundle /absolute/evidence/new-bundle --state-dir /absolute/private/trust
```

All four common options are required. `snapshot` additionally requires a project
relative `--source`; `--claim` and `--next` may repeat. `status` requires
`--verified`. `context` accepts neither that flag nor snapshot-only options.
Unknown, missing, duplicate singleton and positional arguments fail. Each value
is limited to 4096 bytes and claims plus actions to 100 entries.

This bounded deliverable records one successful built-in bytecode compiler
check, never source execution or arbitrary commands. It does not establish test
success, task completion, production compatibility or measured efficiency gains.
`context` returns the authenticated task, claims, remaining actions, observed
check metadata and complete input inventory; it does not automatically select
source excerpts or establish task relevance. Repeated recovery is deterministic
and read-only, including on failure. Failed checks require a fresh bundle.

This initial slice requires a Unix environment with system Git available at
`/usr/bin/git` or `/bin/git`; those trusted absolute locations are used rather
than ambient `PATH`. The root must be the top of an existing Git worktree with HEAD. Inventory covers
actual bytes of every tracked and nonignored untracked regular file, including
staged/unstaged edits, locks, configuration and tests. Tracked deletions use an
explicit `deleted` marker. HEAD and the staged index listing are fingerprinted.
Any difference conservatively invalidates evidence. Files are sorted; files over
16 MiB, more than 20,000 entries or more than 256 MiB of source input fail closed.
The compiler executable (maximum 128 MiB) is hashed; available embedded compiler
revision is recorded, otherwise `unknown`. Binary identity is authoritative.

Ignored and external dependency coverage is unsupported. To prevent unrecorded
include/module dependencies, any source containing the byte substrings `include`
or `use` is rejected before compilation, including occurrences in comments,
strings or longer identifiers. This deliberately conservative restriction is
narrower than the language grammar. Symlinks and special files in the inventory,
source, evidence and trust paths are rejected. External or ignored inputs cannot
be added in this initial version. Installed dependencies and Git metadata beyond
the recorded HEAD/index are outside its coverage.

The check runs with fixed `PATH=/usr/bin:/bin`, `LANG=C`, `LC_ALL=C`, in the project
root, with no inherited environment. Git inventory resolves the executable only
from the fixed absolute `/usr/bin/git`, then `/bin/git` locations, requiring a
regular executable; ambient `PATH` cannot select an inventory program. The manifest records that recipe, normalized
build argv (the ephemeral output directory is represented as `<temporary>`),
logical check argv and successful exit code. Ambient `HOWLFRAME_ALLOW_CAPS` presence
and value are conservatively fingerprinted, without storing plaintext; other
ambient variables are not inputs. The existing 30-second compilation timeout and
1 MiB per-stream output bounds apply. Includes are rejected and source is never
run. Check output is saved separately as `check.json` and hashed.

Bundles and trust directories must be separate and outside the project, and must
have existing parents. Snapshot creates a fresh 0700 bundle and never overwrites
one. A separate private 0700 trust directory contains a random 0600 32-byte key.
The manifest is authenticated with HMAC-SHA256. This detects altered manifests or
evidence without access to that key; hashes alone would not authenticate producer
claims. Snapshot alone may create a key; recovery never does. There is no service,
database, external model call, certificate authority or remote trust claim.

Files are written and synced through temporary names, with the authentication
record published last. Missing publication, check failure, changed inputs during
the check, missing evidence/key, unsupported schema, changed compiler or input,
wrong task/root and tampering fail closed. An interrupted run may leave an
incomplete bundle for inspection; use a new path for the next attempt. Same-user
compromise, replacement of both trust material and evidence, and changes restored
during a check are outside this local guarantee. Filesystem confinement uses
`os.Root` for file reads and rejects static symlink paths; it does not claim
protection against an adversarial process with the same UID racing directory
operations. These constraints are explicit limits, not a complete session or
arbitrary workflow verifier.
