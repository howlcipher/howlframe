# Independent fresh-candidate review, 2026-10-20

Decision: **ACCEPTED as the replacement candidate**. This independently rerun
candidate demonstrates D29 A, C, D, H and standalone operation under G. Final
Milestone 2 closure still requires the committed-tree clean reproduction; this
review does not claim that an untracked candidate was reproduced from a commit.

The evaluator is separate from the application/runtime implementation role in the
same Codex session. No cross-provider independence is claimed. The reviewer wrote
supplemental retained probes and reviewed the final implementation; the implementer
also improved the token crash probe and its transport helper. Authority remains
`milestone2/PLAN.md`, `milestone2/ticketdesk/requirements/REQUIREMENTS.md`,
`requirements/ACCEPTANCE.md`, and `dx/docs_rule.md`. No new milestone gates were
introduced. `git diff 10d9c2a8 --` the frozen plan, requirements, acceptance and DX
paths was empty. The original audit's failures remain recorded separately in
`milestone2/ticketdesk/REASSESSMENT_REVIEW.md`.

## Final independent execution

The reviewer built a separate compiler/artifact after the implementer declared the
candidate stable. These commands completed successfully from the repository root:

```bash
go build -o /tmp/m2-reviewed-final .
/tmp/m2-reviewed-final build milestone2/rebuild/ticketdesk/src/ticketdesk.howl -o /tmp/m2-reviewed-final.hfbc
export HF_BIN=/tmp/m2-reviewed-final APP_ARTIFACT=/tmp/m2-reviewed-final.hfbc
python3 milestone2/ticketdesk/tests/acceptance/run_acceptance.py milestone2/ticketdesk/tests/acceptance/launcher.example.sh
python3 milestone2/ticketdesk/dx/check_dx.py
python3 milestone2/rebuild/ticketdesk/tests/run_reassessment.py milestone2/ticketdesk/tests/acceptance/launcher.example.sh -v
go test -count=1 . -run '^TestHTTPAPIGuideRule$'
git diff --check
```

Results: **101/101 frozen acceptance tests**, **13/13 required DX checks** (one
optional INFO-no), **11/11 retained candidate probes**, and the documentation rule
Go test **passed**. Build and whitespace checks exited 0. Source files were hashed
before and after these executions, with no change:

| File | SHA-256 |
| --- | --- |
| `src/ticketdesk.howl` | `1ebe656ac136f28971c9f3a15765e64cc612c25d61685a1fb43e70e3c8a16567` |
| `migrate_state.py` | `e52e029804db383cec58cb080a417f1eec1f4d2cb519acf8b2e861374b8246bb` |
| `internal/vm/vm.go` (repository root) | `3874435deb15a0f711cfd1a184bfa7c6ebd71a435e57cf1a5a3a8a7756242637` |

HEAD during verification was `9210f20b086690593f9e0ed2351910f706381f42`, with the
candidate and other active milestone work uncommitted. Environment: Linux
`7.2.4-ogc3.1.fc44.x86_64`, Go `go1.26.0 linux/amd64`, Python `3.14.4`. These tests
use temporary data and host tools, not OS sandboxing. Output logs under `/tmp` are
disposable; retained scripts provide the reproducible evidence.

## What the additional probes demonstrate

- Failed startup with an occupied port or missing network preserves existing state
  bytes. Malformed v2 bucket membership, ticket shape, bucket identity and obsolete
  malformed counters fail safely with exit 65 and unchanged input.
- Truncated POST has the exact error shape, correlation id and access line, with
  no mutation. Route, method, authentication and deliberate-fault ordering survive
  interrupted uploads. GET/DELETE ignore stalled declared bodies immediately.
- Default VM budgets handle 150 maximum-length Unicode tickets across buckets,
  pagination and restart. This deliberately removes the frozen harness's raised
  instruction/memory ceilings.
- Six real SIGKILL cycles during eight-way concurrent ticket creation preserve
  every acknowledged record and list/GET consistency. A separate SIGKILL during
  concurrent PATCH/DELETE preserves acknowledged mutations and bucket consistency.
- Three real SIGKILL cycles during token issuance preserve all acknowledged tokens
  and prior confirmed tickets. Each token collection contains order and canonical
  records together; there is no split persisted digest index.
- Offline migration preserves canonical legacy tickets even with every old b:/q:/h:
  index absent, bucket-edge ids, deleted-id counters, token order/roles/revocation
  and authentication after restart. Source remains unchanged; existing output is
  refused; secret strings are absent from migrated state.

Unacknowledged operations may or may not survive a kill. The probes do not require
a particular outcome for them. Legacy-specific index mutation probes remain
historical tests for the old schema and are not applied to v2.

## Source review and resolved findings

Startup parses and validates available raw state before database access, preserving
achievable data-error-before-grant precedence. It validates bucket order, canonical
membership, counters and token records before the write probe. Ticket mutations
publish record and list membership in one atomic store write. Tokens now publish
record and creation order in one atomic collection write; this resolves the
intermediate canonical-token/digest-index gap identified during review.

Authentication hashes the supplied secret and uses `secure_equal` on digests;
VM source maps that to `crypto/subtle.ConstantTimeCompare`. Bootstrap secret is
kept only as a digest in an in-memory store. Issued tokens and ids use `rand_hex`,
backed by `crypto/rand.Read`; runtime request ids use 16 secure random bytes.
Security primitives are verified, without claiming exhaustive side-channel proof
or mathematical absence of random collisions. The `.hfbc` was built from HowlFrame
source and run directly with exactly the five frozen grants, without a generated
helper server. Both runner flags are documented in `docs/cli.md`.

The initially hand-crafted debug response was replaced with a genuine invalid
conversion evaluated inside dispatch; the normal catch path logs its runtime
fault and answers the safe 500. Runtime body prebuffering now propagates incomplete
body errors to actual body access instead of overriding earlier route/auth checks.
Ignored methods bypass buffering, fixing the independently reproduced stall.

## Limits and final closure prerequisite

The preflight port probe still closes its socket before the runner's final bind;
a competing binder can win that small interval. Atomic port reservation is not
claimed. Existing occupied-port behavior and state preservation pass their tests;
this residual race remains a non-blocking carryover under the established scope.

No process-kill test proves power-loss behavior or unlimited workloads. Token
lookup scans active records, and the one-collection representation trades scale
for simple atomic correctness; the milestone has no separate throughput gate.

Run the final committed-tree reproduction after promotion/commit and append its
actual result to the milestone decision/journal. A failed final reproduction
invalidates closure regardless of this candidate acceptance.

## Final committed-tree gate G acceptance, 2026-10-20

**Final decision: ACCEPTED against all selected gates A, C, D, G, H.** The pending
clean-reproduction prerequisite above is now satisfied for implementation commit
`b028e813ac020320c938592193009692973cf079`.

The implementation agent ran `bash milestone2/rebuild/scripts/clean_repro.sh` and
observed process exit 0. The independent reviewer inspected its completed retained
[clean reproduction log](../../../docs/journals/evidence/fresh_milestones/m2_clean.txt),
confirmed it is byte-for-byte identical to the completed execution output, and
re-read the committed script: it archives HEAD into a temporary tree and runs
`env -i` with a fresh HOME, temporary directory, Go build cache and module cache.
The log explicitly identifies commit `b028e813ac020320c938592193009692973cf079`, Go
1.26.0 and Python 3.14.4, followed by **101/101 acceptance tests, 13/13 required DX
checks and 11/11 supplemental candidate probes passing**.

This final clean execution was run by the implementation agent and inspected by
the independent reviewer; the earlier separate rebuild and test executions in
this review were run directly by the reviewer. This accurately distinguishes the
roles rather than claiming an additional independent clean run.

Canonical `milestone2/ticketdesk/src/ticketdesk.howl` and the accepted candidate
both retain SHA-256
`1ebe656ac136f28971c9f3a15765e64cc612c25d61685a1fb43e70e3c8a16567`.
The result uses ordinary host Linux tools and module-download access, with no
claim of OS sandboxing. The known bounded limits above remain recorded; none
blocks the selected milestone gates.
