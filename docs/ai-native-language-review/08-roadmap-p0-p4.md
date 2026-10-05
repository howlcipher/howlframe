# 08: Prioritized roadmap (P0 to P4)

The ordering rule: **first make the existing promise true, then make it
inspectable, then test whether anyone needs it, then grow.** Sizes: S ≤ 2
days, M ≤ 1 week, L > 1 week, for one builder with agent help.

Nothing here flips production `-compile-bc`, marks #90 Done, retakes the
`4d74dbcf` tip-lock, invents DOM APIs, redesigns the language, or changes
the artifact format.

## P0: make the current promise true (now, ~1 to 2 weeks)

| # | Item | Size | Why | Exit criterion |
| --- | --- | --- | --- | --- |
| P0.1 | **Close zero-grant effect paths** | S | Without this the thesis is false | **Done in this PR** for the model-call opcodes (bugs.md #57) |
| P0.2 | **Effect-gate conformance test**: every opcode whose VM case can reach `net/http`, `os`, `os/exec`, or `database/sql` must declare a capability. Ambient opcodes are an explicit allow-list. | S | Stops #44 and #57 from coming back | Test fails if a new effectful opcode has `Capability: ""` (11, C3) |
| P0.3 | **Governed profile v0** (doc + checker flag, no new syntax): a named construct allow-list for model-authored programs. It excludes `lazy_synthesize`, `ephemeral_circuit`, `neural_circuit`, `achieve`, `confidence`, `llm_generate`, `exec`, `spawn`, `db_connect`/`sql_query`, `include` outside root, and all Go/JS targets. | S/M | Stops treating demo primitives as part of the trusted path | `howlframe check --profile governed` rejects the excluded constructs with stable codes (11, C2) |
| P0.4 | **Honest wording**: README, `architecture_roadmap.md`, and `ai_native_paradigms.md` say exactly what "verified" means. Mark `extreme_ai_paradigms.md` as superseded research. Fix the stale "capabilities are advisory" line. | S | Overclaiming is a product risk | Docs PR. No behavior change. |
| P0.5 | **Required-capabilities report** for an artifact (`howlframe inspect` or `-required-caps`): the union of opcode capabilities plus store-URI requirements plus lazy `CALL`, as JSON | S | First "inspect before grant" step. Needed for the experiment. | Sound over-approximation of every VM `requireCapability` site, with tests (11, C1) |
| P0.6 | **Demo apps stop forging and overclaiming**: `release_authority` and `action_executor` emit their decision object with `encode_json` instead of string concatenation (S16). Fix the "failure atomicity" and "security is guaranteed entirely outside the host Go backend" wording in `application_dogfooding_phase_5.md` (S17). | S | These apps are the thesis demos. A forged `ALLOW` in their own output undermines the story. | Quote-injection test: a hostile `target` cannot add a key. Denial tests assert the failure class and that files are unchanged. (11, C7) |

## P1: make execution bounded and checkable (~2 to 4 weeks)

| # | Item | Size | Why |
| --- | --- | --- | --- |
| P1.1 | **Resource limits that exist**: enforce a memory or allocation budget (string, list, and dict growth accounting), `MaxCallDepth` on `CALL`, output-byte cap, `fetch`/`exec` response caps, and a runner wall-clock deadline (context cancellation for host calls) | M/L | S4 to S7. Instruction count alone is not "bounded". |
| P1.2 | **Execution receipt v0**: one JSON line per run with artifact SHA-256, compiler version, grant, budget, instructions used, each effect attempted (kind, target, allowed or denied), exit status | M | Turns "auditable" from a slogan into a file |
| P1.3 | **Verification report v0**: replace the bare `Verified` boolean with named checks, versions, and pass/fail (checker, construct registry, HFIR codes, artifact validation) | S/M | S15 |
| P1.4 | **Decide `and`/`or` semantics** (short-circuit everywhere is the conventional choice) and add a differential test across interpreter, VM, and Go | S/M | S10 changes effects between backends |
| P1.5 | **Run the first falsifiable experiment** (09) | M | Decides whether P2+ is worth doing |
| P1.6 | Regenerate the codegen drift (`SPAWN_AGENT` operand row, orchestrator schema) and move the hand-written artifact section out of the generated file | S | Generated references currently disagree with the code |

## P2: least authority that is useful (after P1.5 says go)

| # | Item | Size |
| --- | --- | --- |
| P2.1 | Resource-scoped grants: `filesystem:read=/data`, `filesystem:write=/out`, `network=host[:port]`, `process=<allow-listed action id>`, `environment=VAR`. Keep the coarse names as aliases. | L |
| P2.2 | Grant attenuation for `SPAWN_AGENT` children. Fold legacy `SPAWN` accounting into the parent budget. | M |
| P2.3 | Full artifact validation (stack-height, operand types, embedded body lengths) for artifacts from outside | L |
| P2.4 | Rooted / hash-pinned `include` and `use` for the governed profile | M |
| P2.5 | Model calls as an explicit `model` capability with provider, model, and token/cost budgets and recorded replay data, separate from `network` | M |

## P3: make the model-facing contract a product (only with consumer demand)

| # | Item | Size |
| --- | --- | --- |
| P3.1 | Wire the HFIR candidate transport and `replace_node` repair to the CLI: `howlframe propose candidate.json`, then `howlframe repair` | M |
| P3.2 | Widen the candidate transport only for constructs the experiment showed users need (likely `for` and a few scoped effects) | M |
| P3.3 | Provenance-tagged values (CaMeL-style) for `req.body`, `fetch`, `read_file`, and model output, plus sink policies | L |
| P3.4 | Signed receipts and artifacts (Sigstore/cosign or minisign) | M |

## P4: later or never

- Production flip of `-compile-bc` to lowered HFIR (stays governed by the
  existing criteria. Not needed for the thesis.)
- More backends, a native Wasm binary pipeline (#92), LLVM.
- New "AI-native" runtime primitives, swarm sophistication, an
  auto-mutating runtime, stochastic control flow.
- DOM/browser API growth and general app-platform features beyond what
  HowlBoard needs.
- Competing with general-purpose languages on libraries or performance.
