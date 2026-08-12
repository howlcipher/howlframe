# Development Notes: Sandboxed Action Executor (Phase 5)

## Architecture
The application separates authorization from execution using a conceptual split. The first half evaluates untrusted JSON proposal inputs against trusted state to produce a normalized execution plan (decision, action_id, normalized_target). The second half takes this execution plan and applies fixed local side effects using HowlFrame's native capabilities.

## Trusted action catalog
A deliberately small whitelist of actions was implemented:
- `read_release_status`
- `run_health_check`
- `stage_artifact`
- `write_release_marker`
- `rollback_marker`

## Trusted vs untrusted inputs
Untrusted inputs (the AI proposal) only specify an action intent, a logical artifact ID, and a reason. Trusted inputs (`tests=PASS`, `approved=yes`, initial `state=...`) are provided by the runner. The untrusted inputs can never override the trusted inputs or policy.

## Capability boundary
Capabilities (`filesystem`, `database`) are controlled entirely by the trusted runner. The proposal cannot escalate privileges. If a proposal attempts to specify `requested_caps: ["network", "process"]`, the runner disregards it. If a legitimate execution effect occurs without the necessary capability granted by the runner, the HowlFrame VM traps and fails closed.

## Filesystem containment
The proposer does not supply raw paths. It specifies a logical ID like `app-v1`. The executor maps this ID to fixed sandbox paths (e.g., `sandbox/fixtures/app-v1.txt`). Arbitrary path traversal like `../../etc/passwd` is evaluated and safely denied. No raw exec calls to shell commands are used.

## Approval model
As in Phase 4, approval is explicitly controlled by the runner via `approved=yes`. AI-provided self-approval flags in the JSON are ignored by the authorization engine.

## State-transition model
Deterministic state transitions are enforced:
- A release marker write can only occur if the system is currently `staged`.
- A rollback can only occur if the system is currently `production_deployed`.

## Failure atomicity
The executor correctly orders its operations: filesystem changes occur first, and only upon success is the simulated `memory://release_state` store updated to reflect the new state. This guarantees that failed side-effects do not falsely report as successful state transitions.

## Adversarial findings
Adversarial tests confirm:
- Proposals cannot invent new executable actions (e.g., `exec` with arbitrary shell commands).
- Path traversals are safely bounded.
- Capability escalation attempts fail without effect.
- Missing capabilities safely halt execution without mutating state.
- Unauthorized state transitions are rejected.

## Compiler/runtime findings
The application perfectly compiles to HowlFrame Bytecode (`.hfbc`) and executes via the VM standalone execution mode. This proves that LLM payloads never touch Go's `exec` layer directly, but are confined safely inside the `.hfbc` sandbox engine.

## Friction
HowlFrame's `try_let` instruction expects the function being invoked to return a value. For side-effecting operations like `write_file` (which don't push a value), attempting to use `try_let` caused a `STACK_UNDERFLOW` error. These functions were properly updated to run directly for their side effects, crashing gracefully if capability policies are violated.

## Bugs
No significant bugs in core were found.

## Developer experience
Building complex state evaluations and branching inside `.howl` is becoming easier, especially when mapping conceptual JSON schemas into deterministic execution plans.
