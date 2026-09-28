# HowlFrame Dogfood Mission 001

**Campaign:** `2026-09-27-continuous-improvement`

## Mission

Use HowlFrame to help govern and improve HowlFrame itself as the trust boundary between autonomous intent and consequential execution.

Primary question:

> Would HowlFrame safely constrain autonomous engineering agents operating continuously without granting them unlimited implicit authority?

This is an execution specification for a future dogfood run. Begin the mission only when explicitly launched with the command in [Launch](#launch). Read this entire document before planning or changing anything. Establish actual implementation before trusting documentation.

## Operating principles

### Ownership and concurrent execution

**INSPECT BROADLY. MODIFY NARROWLY.**

This mission owns only:

- HowlFrame implementation
- plans and policy evaluation
- approval and authorization
- canonicalization and hashing/integrity
- TTL
- apply, verify, and rollback
- replay protection
- execution evidence

The concurrent ecosystem mission owns cross-component architecture, interoperability, shared conventions, contracts, schemas, identifiers, installers, ecosystem integration, cross-component observability, and integration documentation.

The concurrent HowlPlane mission owns HowlPlane implementation, orchestration lifecycle, state, scheduling, task graphs, workers, providers, handoffs, resumability, reconciliation, and orchestration observability.

Inspect related repositories whenever needed, but do not edit another mission's owned component. Record ecosystem- or Plane-specific problems as structured findings with the correct `recommended_owner` instead of making competing edits.

All three missions may run simultaneously. Therefore:

- do not reuse another mission's temporary workspace
- use campaign-, run-, orchestration-, task-, and execution-scoped artifact names
- do not assume a globally exclusive process
- avoid shared unscoped temp files, logs, generated artifacts, state, and locks
- preserve run and execution identity
- surface locking or global-state problems as findings rather than bypassing them

### Evidence before changes

For every candidate defect:

1. establish current behavior,
2. reproduce or demonstrate the problem,
3. preserve evidence,
4. determine expected behavior,
5. make the smallest justified in-scope change,
6. add regression coverage,
7. rerun the exposing scenario, and
8. record before/after results.

Do not perform speculative architecture rewrites merely because an agent prefers another design. Do not manufacture failures to increase finding count.

### Real self-dogfood

This is not a static code review. Where practical, govern a harmless HowlFrame change through HowlFrame itself:

PLAN → EVALUATE → AUTHORIZE → APPLY → VERIFY → preserve evidence

Avoid recursive behavior that could make recovery impossible. When an intended HowlFrame path fails or needs manual intervention, preserve the failure as dogfood evidence before using an escape hatch.

### Severity

Use exactly this vocabulary:

- **P0** — safety/integrity failure that prevents trustworthy autonomous operation
- **P1** — major capability is broken or cannot reliably complete
- **P2** — substantial reliability, architecture, integration, recovery, concurrency, or correctness weakness
- **P3** — meaningful UX, diagnostics, observability, or maintainability issue
- **P4** — optional improvement

**Do not inflate severity.**

### Findings and correlation

Create machine-readable findings as well as Markdown reports. Finding IDs use `HF-*` (for example, `HF-001`). At minimum each finding must conform to this shape; extensions are allowed:

```json
{
  "id": "HF-001",
  "campaign_id": "2026-09-27-continuous-improvement",
  "component": "howlframe",
  "severity": "P2",
  "category": "authorization-integrity",
  "summary": "...",
  "evidence": [],
  "reproduction": [],
  "expected_behavior": "...",
  "recommended_owner": "howlframe",
  "dependencies": [],
  "status": "open"
}
```

Generated evidence should preserve, where feasible, `campaign_id`, `run_id`, `orchestration_id`, `task_id`, `execution_id`, and `parent_id`. Do not invent identifiers the implementation cannot reasonably produce; record missing correlation capability as a finding.

### Git discipline

Before changes, inspect repository contribution practices, branch and worktree state, protected/default branch expectations, and verification instructions. Do not overwrite unrelated work. Do not silently commit directly to a protected/default branch when branches or pull requests are expected. Do not merge work merely to make the report appear successful. Preserve human authority boundaries.

## Investigation scope

Inspect and exercise:

- plan creation
- policy evaluation
- approval and authorization binding
- canonicalization and hashing
- TTL
- APPLY and VERIFY
- rollback
- run and execution identity
- evidence and audit trail
- replay protection
- persistence and restart behavior
- concurrency
- CLI behavior and error semantics

Record exact source revisions, configuration, commands, inputs, outputs, identifiers, artifact paths, digests, times/clock controls, exit statuses, and operator interventions.

## Lifecycle and trust model

Determine the implemented lifecycle from source and executable behavior. Evaluate whether Frame has a clear lifecycle conceptually similar to:

PLAN → EVALUATE → AUTHORIZE → APPLY → VERIFY → COMPLETE

with explicit alternate outcomes such as REJECT, EXPIRE, FAIL, ROLLBACK, and REVISE. Do not force these exact names if existing semantics are superior, but require explicit, deterministic, persisted lifecycle behavior. Identify legal transitions, required data, authority, side effects, restart semantics, terminal states, and invalid-transition handling.

## Plan and authorization integrity

Determine exactly what approval binds to. Investigate whether authorization covers:

- canonical plan
- artifacts and their digests
- parameters
- target and environment
- scope and capabilities
- policy identity/version
- actor or authorizing mechanism
- TTL and authorization time
- expected verification criteria

A materially changed operation must not silently inherit authorization. Safely test plan mutation, artifact mutation, parameter mutation, target mutation, environment mutation, copied approvals, replay, concurrent executions, and expired approvals.

## Hashing and canonicalization

Audit:

- canonicalization rules and deterministic representation
- exact data hashed and omitted fields
- path normalization and path identity
- metadata behavior
- line-ending behavior
- ordering and serialization stability
- pre-apply revalidation
- recorded final digest

Use equivalent and non-equivalent fixtures to distinguish harmless representation changes from material changes. The operation actually applied must match the authorized digest.

## TTL

Use deterministic or injectable time where feasible. Cover:

- valid authorization
- just before expiry
- exact expiry boundary
- immediately after expiry
- resume after expiry
- clock source and timezone assumptions
- clock rollback/advance behavior where safely testable

Avoid flaky sleeps when a controllable clock is practical. Record boundary semantics explicitly.

## Apply

Evaluate:

- stale-plan prevention
- authorization and execution correlation
- scope and capability enforcement
- process interruption and restart behavior
- observability and durable evidence
- time-of-check/time-of-use weaknesses
- duplicate APPLY behavior

Use harmless fixtures and isolated targets. Do not perform destructive tests merely to demonstrate theoretical risk.

## Verify

Verification must be meaningful and bound to the authorized operation. Test:

- skipped verification
- empty verification
- failed verification
- verification associated with the wrong execution
- mutation of verification criteria after authorization
- repeated VERIFY
- verification after expiry or invalid state

A successful final state must not be reachable through a verification bypass.

## Rollback

Use safe, reversible fixtures. Test:

- correct prior-state selection
- run and execution scoping
- concurrent isolation
- rollback evidence
- rollback failure
- repeated rollback
- rollback from invalid states
- rollback against another execution
- whether consequential rollback itself requires governance

Rollback must not cross execution boundaries or obscure the fact that APPLY occurred.

## Replay and idempotency

Test duplicate APPLY, repeated VERIFY, reused approval, old execution ID, repeated ROLLBACK, and process interruption/restart. Determine which operations are rejected, idempotent, or require a new authorization. Look for duplicate side effects, state corruption, evidence overwrite, and stale authorization reuse.

## Concurrency

Run independent Frame executions concurrently using isolated harmless fixtures. Look for:

- approval crossover
- run-state leakage
- shared temporary files
- incorrect or overwritten hashes
- evidence crossover
- rollback crossover
- race conditions
- unscoped locks

Also test same-target contention where safely representable, and verify deterministic conflict or serialization behavior rather than silent last-writer wins.

## Adversarial safe testing

Using only harmless fixtures, attempt:

- post-approval artifact mutation
- plan argument mutation
- target substitution
- old approval replay
- execution-ID substitution
- evidence tampering
- APPLY from an invalid lifecycle state
- VERIFY bypass
- rollback against another execution

Preserve both rejected and unexpectedly accepted attempts. Do not weaken safety controls, use real secrets, or create consequential side effects.

## Evidence and auditability

A completed Frame run must allow reconstruction of:

- proposed operation
- exact authorization
- applicable policy and version
- authorizing actor or mechanism
- authorization issuance and expiration
- artifacts and digests
- operation actually applied
- verification criteria and results
- rollback, if any
- final state and transition history

Evaluate tamper evidence, persistence, correlation, redaction, schema versioning, and whether independent review can reproduce the decision.

## Remediation and verification

Only remediate validated HowlFrame-owned defects. Every fix requires:

- preserved reproduction
- defined expected behavior
- minimal correction
- regression test at the appropriate tier
- rerun of the negative case
- rerun of the success path
- before/after evidence from the final working tree

After remediation, perform an independent audit intended to falsify correctness. Reconcile all findings without silent dismissal. Record unresolved, disputed, out-of-scope, and human-decision findings explicitly.

## Required outputs

Place version-controlled reports under `docs/` or clearly run-scoped artifacts under the established run-artifact location:

- `HOWLFRAME_DOGFOOD_REPORT.md`
- `HOWLFRAME_FINDINGS.json`
- `HOWLFRAME_TRUST_MODEL.md`

The final report must contain:

1. Executive Summary
2. Current Trust Model
3. Lifecycle
4. Plan Integrity
5. Artifact Integrity
6. Authorization Boundary
7. TTL Testing
8. Apply Testing
9. Verification Testing
10. Rollback Testing
11. Replay Testing
12. Concurrency Testing
13. Adversarial Testing
14. Evidence/Audit Findings
15. Defects
16. Fixes
17. Regression Coverage
18. Before/After Results
19. Remaining Risks
20. Recommended Remediation Waves

Optimize for bounded, explainable, recoverable, and verifiable autonomy.

## Completion criteria

The mission is complete only when:

- the implemented trust model and lifecycle are evidenced
- authorization binding, canonicalization, hashing, TTL, apply, verify, rollback, replay, restart, and concurrency are exercised safely
- a meaningful self-governed path is attempted where practical
- failures and manual interventions are preserved
- all findings are valid machine-readable records with correct ownership
- each in-scope fix has regression coverage and before/after rerun evidence
- an independent audit challenged the results
- no ecosystem- or Plane-owned implementation was modified

## Successor Mission Contract

After this mission has completed:

1. implementation,
2. deterministic verification,
3. independent audit,
4. remediation of valid in-scope findings,
5. verification after remediation,
6. final acceptance,
7. findings reconciliation,

the agent must decide whether meaningful follow-up work remains.

Do not create another mission merely because iteration is possible, for cosmetic churn, or to keep agents busy. Create `docs/DOGFOOD_MISSION_002.md` only when one or more of the following is true:

- an unresolved P0 finding exists
- an unresolved P1 finding exists
- meaningful P2 work remains
- multiple P3 findings share a root cause worth addressing
- completed remediation exposes another necessary maturity step
- deterministic evidence demonstrates a missing capability
- a blocked capability becomes actionable
- architecture needs a bounded follow-up before the system can progress safely

If no worthwhile successor exists, record in the final completion report:

```
NEXT_MISSION: NOT_REQUIRED
```

If meaningful work remains, create `docs/DOGFOOD_MISSION_002.md`. The successor must be derived from Mission 001 evidence, must not merely duplicate Mission 001, and must explicitly identify:

- predecessor mission (`docs/DOGFOOD_MISSION_001.md`)
- campaign ID (`2026-09-27-continuous-improvement`)
- findings that justify it, citing finding identifiers as evidence whenever possible
- work completed by the predecessor
- work that must not be repeated
- unresolved findings
- newly exposed findings
- dependencies
- repository ownership
- objective
- scope
- explicit non-goals
- deterministic verification
- independent audit requirements
- success criteria
- stop conditions

Mission numbering applies recursively to future missions. A Mission N may generate Mission N+1 when justified (for example, `DOGFOOD_MISSION_002.md` may generate `DOGFOOD_MISSION_003.md`). Each successor must reference its predecessor. Never overwrite previous mission documents; they are historical execution contracts.

Creating a successor mission does not authorize executing it. Do not invoke `howl orchestrate` recursively, `howl factory` recursively, or launch Mission N+1 from Mission N. Return control to the outer Factory supervisor after Mission N completes.

### Factory handoff

The final completion report for this mission must end with a machine-readable final section containing:

```
MISSION_STATUS: <COMPLETE|BLOCKED|IDLE>
NEXT_MISSION: <docs/DOGFOOD_MISSION_002.md or NOT_REQUIRED>
OPEN_P0: <count>
OPEN_P1: <count>
OPEN_P2: <count>
OPEN_P3: <count>
BLOCKED: <true|false>
RECOMMENDED_PRIORITY: <advisory only>
FACTORY_NOTES: <any portfolio context>
```

`RECOMMENDED_PRIORITY` is advisory only; the master Factory makes the final portfolio decision. A project is allowed to report that its own next mission should not run yet.

## Launch

Run from this repository only when ready to begin the mission:

```bash
howl orchestrate \
  "Execute HowlFrame Dogfood Mission 001 exactly as specified in docs/DOGFOOD_MISSION_001.md. Read the entire mission before planning. Work within the ownership boundaries defined there. Dogfood HowlFrame as its own governed execution boundary, remediate validated in-scope findings, verify them deterministically, perform independent audit, and produce the required final evidence." \
  --repo "$PWD" \
  --heartbeat 15
```
