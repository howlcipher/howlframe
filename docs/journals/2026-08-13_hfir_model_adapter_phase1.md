# Goal

Prove or disprove one provider-neutral, model-authored HFIR program can pass
deterministic validation, direct HFIR-to-bytecode lowering, isolated VM
execution, and a bounded semantic repair without producing `.howl` or an AST.

# Starting SHA

`f39f76049b9354008645b7bd5116a8112a4923b5` (`origin/main`, verified live).

# Hypothesis

A model can author only the executable Phase-1 HFIR transport, receive stable
structured diagnostics, and repair a declared local semantic region without
controlling authority or backend details.

# Existing executable HFIR subset

`program`, `sequence`, `let`, `set`, `if`, `const`, `symbol`, `binary`,
`list`, `dict`, `dict_entry`, conversions, list/map/string operations,
`print`, `stderr`, `exit`, and `env`, as recorded in
`docs/hfir_execution_status.md`.

# Current adapter/orchestrator state

No HFIR adapter exists. The optional orchestrator emits bytecode through an
OpenAI-compatible local experiment, so it is explicitly outside this path.

# Trust boundary

The model proposes semantic structure and constants only. The trusted caller
selects capability grants and instruction policy. Schema validation, graph
validation, verification, opcode selection, artifact validation, execution,
and behavioral oracles are deterministic-owned.

# Model-owned data

Versioned transport graph node IDs, supported node kinds, input references,
semantic values, literal kinds, and bounded provenance labels.

# Deterministic-owned data

Schema rules, graph version acceptance, decoder limits, canonical graph hash,
capability effects and grants, execution policy, verifier/lowerer rules,
bytecode opcodes, artifact acceptance, and all test oracles.

# Non-goals

No provider implementation, public build-path switch, AST fallback, broad
HFIR coverage, content-addressed storage, full patch system, HTTP, stores,
functions, loops, recovery, or experience memory.

## Task

Black-box HFIR synthesis benchmark: eleven compact Phase-1 tasks covering
arithmetic, branches, list/map mutation, strings, `env`, stderr/exit, nested
logic, and a deliberately wrong greeting for repair.

## Model input

Only `docs/reference/hfir_model_adapter_phase1.schema.json`,
`docs/hfir_model_adapter_status.md`, and the natural-language task list were
provided to the restricted author role. It was explicitly barred from source,
AST, HFIR implementation, bytecode, tests, and fixtures.

## Candidate HFIR result

The recorded result is
`docs/fixtures/hfir_model_adapter_black_box_phase1.json`: 11 candidates,
average 8.64 nodes (95 nodes total). Ten were intended to pass first attempt;
the greeting candidate deliberately printed `helo` for the repair experiment.

## Verification result

All 11 candidates passed strict transport schema/integrity validation and the
model-boundary compile gate. All went through direct `LowerToBytecode`, HFBC
artifact validation, and isolated VM behavioral tests. No `.howl`, AST, legacy
compiler, or target source was used in this pipeline.

## Diagnostics

The intentional greeting failure was behavioral, not structural: expected
`hello\n`, observed `helo\n`; the trusted target was node `greeting`.
Adversarial coverage rejects malformed JSON, duplicate fields, unknown fields,
unsupported functions, capability/budget self-grants, raw Go injection, wrong
operand roles, cycles, and malformed graph versions before a BCProgram exists.

## Repair requested

The bounded context contained the graph and node precondition hashes, target
`greeting`, its direct neighborhood, and one permitted `replace_node` action.

## Repair size

One replacement node (1 of 3 nodes; 33.3% for this intentionally tiny graph),
changing only the string constant `helo` to `hello`. The original black-box
author emitted an outdated delta shape; deterministic rejection was correct.
The recorded repair fixture used the published hash-guarded delta contract.

## Final result

Re-decode, verification, direct lowering, artifact validation, and VM oracle
passed with `hello\n`. Repairs are rejected if stale, outside the trusted
target, identity-changing, capability-introducing, or reference a node outside
the trusted direct-input set.

## What this taught us

The small direct subset is sufficient for a real model-to-execution and local
repair loop. Exact machine-readable repair contracts matter: an otherwise
valid author can use stale field names. Schema-invalid attempts require full
regeneration because no canonical graph/hash exists yet.

## Task

Initial `.howl` baseline attempt by a separately restricted author role.

## Model input

Natural-language tasks only; no compiler internals or source fixtures.

## Candidate HFIR result

Not applicable. Its source candidates are retained in
`docs/fixtures/hfir_model_adapter_howl_baseline_phase1.json`.

## Verification result

0/10 compiled through the existing AST `-compile-bc` workflow. Live compiler
diagnostics included missing `cli_app`, unsupported surface names, and invalid
binding forms.

## Diagnostics

Compiler diagnostics were concise but source/location oriented, rather than
node-addressable semantic repair targets.

## Repair requested

None for the initial attempt; it is preserved as negative evidence.

## Repair size

Not measurable; all attempts required full-source regeneration.

## Final result

Baseline first attempt failed closed before execution.

## What this taught us

The initial baseline is not a fair measure of `.howl` expressiveness. It is a
useful measure of authoring without a source construct reference only.

## Task

Repeat the `.howl` baseline for the same ten non-repair observables after
providing only public `README.md` and `docs/cli.md` source documentation.

## Model input

Natural-language tasks plus those two public source documents. No source
fixtures, AST/compiler code, HFIR code, bytecode, or tests were available.

## Candidate HFIR result

Not applicable. The retry source candidates are retained in
`docs/fixtures/hfir_model_adapter_howl_baseline_retry_phase1.json`.

## Verification result

3/10 AST bytecode compiles passed (`arithmetic`, `classify`, `nested-if`), and
all three produced the required observed output through `-run`. The other 7
failed closed before execution: three used invalid multi-expression `let`
bodies, and four used unsupported/publicly misidentified names (`get_env`,
`eprint`, `length`, `join`). This is a source baseline measurement, not a
claim that the existing source language lacks equivalent forms.

## Diagnostics

The public AST path gave structured source location diagnostics, but they did
not identify a model-editable semantic node or a bounded graph neighborhood.

## Repair requested

None. This comparison measures one full-source generation attempt per task.

## Repair size

No deltas exist on the source path; every retry would be full representation
regeneration.

## Final result

HFIR: 10/10 non-deliberately-wrong task candidates behaviorally passed on the
first graph attempt, plus 1/1 bounded repair. Documented `.howl` retry: 3/10
behaviorally passed on first source attempt.

## What this taught us

For this restricted author and compact subset, explicit semantic roles and
schema produced a stronger first-pass result than public source syntax. The
sample is too small and source-documentation-sensitive to establish a general
provider-quality claim.
