# Application Dogfooding Phase 4

This phase implements the Release Authority demo, demonstrating HowlFrame's central architectural claim: **Intent is not authority.**

## Did HowlFrame successfully separate intent from authority?
**Yes.** The untrusted intent is securely sequestered as a parsed JSON payload, while the authority is derived entirely from trusted runtime evidence and rigid bytecode-enforced logic.

## Could an AI self-approve?
**No.** Self-approval fields provided in the proposal (`"approved": "true"`) are bypassed and have absolutely no impact on the policy logic.

## Could proposal content widen runtime capabilities?
**No.** Asking for broad capabilities (e.g., `"requested_caps": ["database", "network"]`) in the JSON proposal does not grant the authoritative runtime any additional permissions. If the runner does not grant the `database` capability, attempts to mutate state will panic and fail closed, regardless of the proposal's requests.

## Could confidence override deterministic policy?
**No.** High confidence flags (`"confidence": "1.0"`) are ignored. Hard deterministic evidence (e.g., `tests=PASS`) holds final say over all decisions.

## Could prompt-injection-like text bypass policy?
**No.** Hostile instructions loaded into the `"reason"` field were parsed safely as inert strings and did not alter execution.

## Did denied proposals mutate state?
**No.** Denied operations reliably fail to mutate the simulated store, exiting cleanly with zero side-effects.

## Did approval remain runner-controlled?
**Yes.** All approval is supplied via CLI arguments by the trusted runner.

## Did standalone bytecode enforce the policy?
**Yes.** The application successfully compiles to `.hfbc` and runs via `howlframe -run-bc` in complete isolation.

## Was live AI integration necessary?
**No.** The live proposer integration was deliberately omitted. Pushing AI generation into a separate out-of-process orchestrator produces a much cleaner trust boundary and removes the need to grant network access to the authoritative evaluator.

## What architectural gap appeared?
No blocking architectural gaps appeared. HowlFrame successfully provided all the primitives needed to implement deterministic, capability-gated policy around untrusted data.

---

## Architecture
The application runs as a standalone bytecode CLI. It reads an untrusted JSON file containing the AI's proposal, parses it into an opaque map, and then matches the action against a set of hardcoded policies evaluated with trusted runner-supplied arguments.

## Proposal schema
```json
{
  "action": "deploy_production",
  "target": "production",
  "reason": "Tests passed.",
  "confidence": "0.95"
}
```

## Trusted evidence
Evidence is passed via CLI arguments such as `tests=PASS`, `security=PASS`, and `change_window=OPEN`. These form the objective truth that the policy evaluates against.

## Approval mechanism
Approval is explicitly runner-controlled via the `approved=yes` argument.

## Decision policy
Policy evaluates the deterministic evidence:
- **inspect**: unconditionally `ALLOW`
- **deploy_staging**: `ALLOW` if tests/security pass and approved; `REQUIRE_APPROVAL` if missing approval; else `DENY`
- **deploy_production**: `ALLOW` if all evidence passes and approved; `REQUIRE_APPROVAL` if missing approval; else `DENY`
- **rollback**: `ALLOW` if approved; else `REQUIRE_APPROVAL`

## Capability boundary
The core evaluator only needs the `filesystem` (to read the proposal) and `database` (to mutate the simulated state store) capabilities. It safely isolates state effects and prevents the proposer from escalating permissions. 

## State transition model
Simulated release states (`staging_deployed`, `production_deployed`, `rolled_back`) are tracked in the `memory://release_state` native store. Mutation only occurs when the decision is strictly `ALLOW`.

## Adversarial results
- **Self approval**: Overrides ignored, safely blocked.
- **Capability escalation attempt**: Fails closed, ignored.
- **Prompt-injection-style reason**: Parsed as string data, safely evaluated without executing.
- **Confidence override attempt**: Failed evidence led to `DENY` despite high confidence.
- **Malformed proposal**: Fail closed safely via `try_let`.

## Standalone bytecode result
The release authority runs successfully as a `.hfbc` bytecode artifact, fully independent from the Go backend.

## Live AI integration
Live AI integration was deliberately omitted from the authoritative evaluator to maintain a clean trust boundary. The AI proposer should remain an external, non-authoritative caller.

## Core changes
No HowlFrame core changes were required.

## Recommendation
> READY FOR SANDBOXED ACTION EXECUTOR
