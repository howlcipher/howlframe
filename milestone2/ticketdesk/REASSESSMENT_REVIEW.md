# Independent Milestone 2 reassessment, 2026-10-20

Decision: **baseline not accepted on fresh review; replacement awaits independent evaluation**.
The prior acceptance remains historical evidence, not this review's conclusion.
The evaluator did not implement the application or runtime fixes. This is a separate
agent role in the same Codex session, without evidence of provider independence.

## Authority and method

Read `milestone2/PLAN.md`, frozen `requirements/REQUIREMENTS.md`,
`requirements/ACCEPTANCE.md`, and `dx/docs_rule.md` before application/runtime
inspection. Evaluated only D29 A, C, D, G, H. Authored and retained independent
supplemental checks in `tests/reassessment/run_reassessment.py`, reusing the frozen
process/HTTP harness but no frozen test assertions. Frozen requirements, acceptance
and DX files were not edited by this evaluator. `git diff 7092bcff --` those paths
was empty. No application or runtime implementation was performed by this reviewer.

Build and baseline execution (uncommitted concurrent changes existed outside M2;
HEAD at observation was `e6fdd48a0c60564a2ad3833cf2b990dff7ad7c14`):

```bash
go build -o /tmp/m2-reassessment-hf .
/tmp/m2-reassessment-hf build milestone2/ticketdesk/src/ticketdesk.howl -o /tmp/m2-reassessment.hfbc
export HF_BIN=/tmp/m2-reassessment-hf APP_ARTIFACT=/tmp/m2-reassessment.hfbc
python3 milestone2/ticketdesk/tests/acceptance/run_acceptance.py milestone2/ticketdesk/tests/acceptance/launcher.example.sh
python3 milestone2/ticketdesk/dx/check_dx.py
python3 milestone2/ticketdesk/tests/reassessment/run_reassessment.py milestone2/ticketdesk/tests/acceptance/launcher.example.sh -v
```

Fresh results: frozen acceptance **101/101 pass**; DX **13/13 required checks pass**
(one optional diagnostic check reports INFO-no). New probes: **1/7 passes**, six
failures, reproduced before root M2 changes. Host: Linux
`7.2.4-ogc3.1.fc44.x86_64`, Go `go1.26.0 linux/amd64`, Python `3.14.4`.
These executions use temporary data and ordinary host tools, not OS sandboxing.
Baseline stdout logs under `/tmp` are disposable; retained scripts allow reruns.

## Reproducible findings

1. **R1.3 failed-startup mutation.** With an occupied port and an existing zero-byte
   valid state, startup exits nonzero but rewrites the state. With missing network
   grant and existing `{}\n`, startup exits 77 but rewrites it. `prove_write` precedes
   both checks. Existing data must remain unchanged on failed startup.
2. **R1.3 invalid state accepted.** Counter `next_ticket: 0` is accepted and listens
   instead of exit 65. Baseline `prove_write` checks integer encoding only. The
   retained case also includes a negative token counter and truncated ticket record;
   execution currently stops at the first invalid counter, so those later cases
   are authored assertions, not claimed baseline observations.
3. **R3.3/R7.1/R8/R16 truncated transport bypass.** A half-closed POST with a declared
   body longer than the delivered valid JSON returns 400 with no `X-Request-Id`,
   and `error` is a string instead of the required nested object/code. Source
   inspection shows the early runtime return also bypasses access logging.
   The retained test asserts schema, request id, access line and absence of mutation.
4. **Supplemental legacy recovery weakness.** Removing all bucket records from a
   valid legacy state loses confirmed tickets from listing while GET still reads
   them. Removing ids 1/64/65 from buckets likewise loses those records from listing.
   These are deterministic simulated intermediate states, not a claimed newly run
   kill-storm. Recovery compatibility is supplemental coverage, not an additional
   milestone gate. A replacement using a new schema may explicitly decline old
   schema migration; these probes must then be kept as historical legacy-specific
   tests rather than silently treated as new-schema failures.
5. Dangling index recovery (ticket record removed, index id retained) **passes**,
   including paged listing, further creation, and a second SIGKILL restart.

## Reviewer-verified security items

Baseline source authenticates by SHA-256 digests and `secure_equal`; the VM uses
`crypto/subtle.ConstantTimeCompare`. Issued secrets and ids use `rand_hex`, backed
by `crypto/rand.Read`. Runtime request ids use 16 secure random bytes. This verifies
use of the intended primitives, not a mathematical guarantee against random
collisions or exhaustive side-channel analysis. Artifact was built from `.howl`
and invoked directly through the standalone runner; no generated helper server.

## Remaining decision

Root is authoring a fresh replacement under `milestone2/rebuild/`. This document
records the completed baseline audit only. Independently rebuild and evaluate
that final candidate against the frozen suite, DX/docs rule, applicable retained
startup/transport assertions, new-schema consistency probes and clean reproduction
before accepting it. Do not interpret this pending review as acceptance.
