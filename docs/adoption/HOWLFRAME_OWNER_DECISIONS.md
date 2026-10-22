# HowlFrame Owner decision packet

Prepared 2026-10-21 by the implementing agent (Claude Opus 5.5). **This document records recommendations and evidence.
It records no approval.** The mission brief that commissioned it proposed resolutions for A–E; those are treated
here as recommendations to verify, not as decisions, because the brief itself requires approvals to be recorded
separately. When the Owner decides, record the decision, date and wording in the "Decision" row of each item.

| # | Decision | Recommendation | Status |
|---|---|---|---|
| A | Minimum Go toolchain | Keep Go 1.25 | **Open** |
| B | Required GitHub checks | Require `build`, `acceptance`, `race` on HowlFrame `main`; protect HowlBoard `main` | **Open** (needs admin action) |
| C | Milestone 3 verdict wording | D30 A demonstrated; D30 B compatibility demonstrated, adoption pending; D30 J demonstrated | **Open** |
| D | R5 adapter addendum | Supersede; retire as a current requirement, keep the historical record | **Open** |
| E | Process-security limitations and #181 | Record the three-level distinction; #181's own criteria are met; track the hostile case separately | **Open** |
| F | HowlBoard ledger-import behavior change (HowlBoard #20) | Approve | **Open** |
| G | HF-05 checker/adapter tightening (HowlFrame #212) | Approve after one independent confirmation of the final head | **Open** |
| H | `process:env` policy (#182) | Adopt the classified policy in `HOWLFRAME_CORRECTNESS_FOLLOWUPS.md` §2, in two steps | **Open** |
| I | Competitive pilot: providers, H2/H4 options, budget | Option A for H2, default for H4, a fixed cap; one more protocol re-review first | **Open** |

No PR has been merged, no branch protection changed, and no issue closed in this mission.

---

## A. Minimum Go toolchain

**Evidence (verified 2026-10-21):**
- `go.mod` declares `go 1.25`; the CI `build`, `acceptance` and `race` jobs and `release.yml` use 1.25; `latest-go` uses stable.
- HF-01's fix needs `os.Root.MkdirAll`, `Root.Rename` and `Root.Chmod`, which were added in Go 1.25. With `go 1.24`, `internal/capability`
  fails to compile: `*os.Root does not implement FileSystem (missing method MkdirAll)`.
- With Go 1.25.0 exactly, all five release targets in `release.yml` cross-build (`linux/amd64`, `linux/arm64`, `darwin/amd64`,
  `darwin/arm64`, `windows/amd64`, `CGO_ENABLED=0`).
- Windows and macOS binaries are **built**, never executed, in any evidence to date.
- `go test ./...` passes on 1.25.0 (the #209 regression record; also part of the CI `build` job).

**Alternatives considered:** Go 1.24 cannot compile the race-free filesystem layer. Hand-written `openat2`/`O_NOFOLLOW` walkers
would allow 1.21, but they are Linux-only (no parity for the shipped macOS/Windows builds) and were rejected in #209 for that reason.
No equally safe, simpler option was found.

**Recommendation:** keep Go 1.25 as the minimum. Fix a stale library entry: `docs/installation.md` still says "Go 1.21 or newer".
That holds only with `GOTOOLCHAIN=auto` (the default), which downloads 1.25. With `GOTOOLCHAIN=local` and an older Go, the build fails.

**Decision:** _(Owner)_

## B. Required GitHub checks

**Evidence (live API, 2026-10-21):**
- HowlFrame `main` protection: required contexts `["build"]` (GitHub Actions app 15368), `strict: false`, admins not enforced.
- CI job names on PRs and `main`: `build`, `acceptance`, `race`, `latest-go`. All four passed on `bfc4ff8f`, and on PRs #211 and #212.
- On `main` pushes there are **two** check runs named `build` (CI and the Pages workflow), so the single required context is ambiguous on
  `main`. PRs run only CI.
- HowlBoard `main` has **no** branch protection (`404 Branch not protected`). Its CI job is
  `Build, Contract Tests, Docs & SEO Verification`.

**Recommendation:**
1. HowlFrame: require `build`, `acceptance` and `race`, with `strict: true` (branch up to date before merge). Leave `latest-go` advisory,
   since a new Go release can break it without any change in the PR.
2. HowlBoard: require `Build, Contract Tests, Docs & SEO Verification`.
3. Optionally rename the CI `build` job (for example `ci-build`) to remove the clash with the Pages job, and update the required context in the same change.

Exact command for the Owner (needs admin rights; **not run**):

```bash
gh api -X PATCH repos/howlcipher/howlframe/branches/main/protection/required_status_checks \
  -F strict=true -f 'contexts[]=build' -f 'contexts[]=acceptance' -f 'contexts[]=race'
gh api -X PUT repos/howlcipher/howlboard/branches/main/protection --input - <<'EOF'
{"required_status_checks":{"strict":true,"contexts":["Build, Contract Tests, Docs & SEO Verification"]},
 "enforce_admins":false,"required_pull_request_reviews":null,"restrictions":null}
EOF
```

**Decision:** _(Owner)_

## C. Milestone 3 verdict wording (D30 A, B, J)

**Evidence:**
- D30 A (full-stack application): EvidenceDesk's API, persistence, real-Chromium and XSS workflows pass. They were reproduced by the
  2026-10-21 audit, and again in this mission on `main` and on #211 (`evidence/08_*`).
- D30 B (useful ecosystem component): **no adoption on any merged branch.** HowlBoard `main` (`f4c7bb5`) has zero references to EvidenceDesk.
  This mission measured the projector against HowlBoard's incumbent importer. As a replacement it is a regression: it fails 8 of HowlBoard's own
  verification-truth assertions, including rewriting a failed check's exit code 7 as 0 (`evidence/03_truth_contract.txt`). What *is* adoptable
  is EvidenceDesk's ledger **validation module**. HowlBoard PR #20 (open, unmerged) adopts it.
- D30 J (evidence-based sequencing): recorded in the M3 plan and decision records.
- New: EvidenceDesk's own `verify_workflows.py` **fails against current upstream HowlBoard** on `main`. Its mandatory R5 adapter step inserts
  a second `(grants ...)` form, which current Board rejects. It passes only against the stale `cd2f2fd` checkout
  (`evidence/07_*`, `evidence/08_*`). So the frozen R5 "actual consumer" check is no longer reproducible against the current consumer as written.

**Recommendation:** record, as a dated correction beside (not replacing) `MILESTONE3_DECISION.md`:
- D30 A: **demonstrated.**
- D30 B: **compatibility demonstrated; adoption pending.** If HowlBoard #20 merges, it becomes "partially adopted: HowlBoard's
  production import path validates input with the EvidenceDesk ledger module; the EvidenceDesk projector is not used."
  The projector itself should not be described as a replacement.
- D30 J: **demonstrated.**

Historical ACCEPTED records stay as written. The correction text belongs in a new dated file, for example
`milestone3/CORRECTION_<date>.md`, once the Owner chooses the wording. It is not written here, to avoid pre-empting the decision.

**Decision:** _(Owner)_

## D. R5 adapter addendum

**Evidence:** the addendum (`milestone3/evidencedesk/requirements/R5_COMPATIBILITY_ADDENDUM.md`) was approved by the implementing orchestrator, not
the Owner. It was written against the stale checkout. Current upstream needs no adapter under its declared grants (`EVIDENCE_STATEMENT_2026-10-21.md`).
Applying the adapter to current upstream is now a **compile error** (this mission, `evidence/07_*`).

**Recommendation:** supersede the addendum as a current compatibility requirement, and keep it and `board_unadapted_failure.txt` as
historical evidence. Do not record it as Owner-approved. As a follow-up (needs approval, since it touches a frozen verification script):
make `verify_workflows.py` apply the adapter only when the consumer declares no root `(grants ...)`, and otherwise run the unmodified
consumer under its own declared grants. That is the configuration current upstream actually uses.

**Decision:** _(Owner)_

## E. Process-security limitations and issue #181

Three different properties, only the first of which is implemented:

| Property | Meaning | Status |
|---|---|---|
| Runaway process containment | When an `exec` call returns (normal exit, deadline, output limit), nothing it started keeps running, including background and `setsid` descendants | **Implemented (Linux)** in #209: a supervisor with `PR_SET_CHILD_SUBREAPER`. Tested by `TestExecKillsBackgroundAndSessionEscapedDescendantsOnNormalExit`, `TestExecDeadlineKillsEveryDescendant` and `TestExecOutputLimitKillsEveryDescendant`; independently verified in the #209 review. Weaker on macOS/Windows (process-group kill only) |
| Adversarial process isolation | Code that *wants* to escape cannot, for example by killing its supervisor or racing the sweep | **Not implemented.** A child that kills its own supervisor can leave detached descendants (documented in `docs/remediation/HOWLFRAME_SECURITY_REMEDIATION.md`) |
| Complete OS sandboxing | Namespaces, seccomp, cgroups, filesystem and network isolation of untrusted code | **Not implemented and not claimed** (`SECURITY.md`) |

**#181's own criterion** (issue text): "setsid and background exec children survive the deadline. Fix (process group, subreaper or cgroup kill)
before any live run grants process:exec." The implemented subreaper meets that criterion for the cases it names, with tests.

**Recommendation:** close #181 with a comment citing the three tests, and open a new issue for the adversarial case (supervisor kill;
design noted in the #209 handoff as "runner-as-second-subreaper"). Do not describe HowlFrame as isolating hostile code.
Not done here: closing an issue is the Owner's call.

**Decision:** _(Owner)_

## F. HowlBoard ledger-import behavior change (HowlBoard #20)

On valid input the output is unchanged (golden file plus 7-case differential). On input the previous importer mishandled, it now refuses with a line
number instead of skipping, double-counting, truncating, crashing, or (empty file) wiping the timeline. `make import` also raises the VM memory
charge to 256 MiB, which is not a grant. Full table: `HOWLFRAME_ADOPTION_REPORT.md` §4.

**Recommendation:** approve and merge after the independent review findings are resolved. **Decision:** _(Owner)_

## G. HF-05 tightening (HowlFrame #212)

It rejects programs that could never run correctly. One reference program changed (it leaked a secret and then crashed when granted). Three review
rounds each found a variant of the same root cause, and all are fixed. The final head `7de10d4a` is implementer-verified only. Details:
`HOWLFRAME_CORRECTNESS_FOLLOWUPS.md` §1. **Recommendation:** approve after one independent confirmation of `7de10d4a`
(for example, Antigravity re-running its round-3 brief). **Decision:** _(Owner)_

## H. `process:env` policy (#182)

A proposal with measured consumer impact is in `HOWLFRAME_CORRECTNESS_FOLLOWUPS.md` §2. Nothing is implemented.
**Recommendation:** approve step 1 (classified refusal of execution-steering families, no breakage measured); decide step 2
(allowlist by default with an explicit opt-in for application names) separately. **Decision:** _(Owner)_

## I. Competitive pilot

The protocol (`experiments/competitive-v1/HOWLFRAME_COMPETITIVE_PROTOCOL.md`, PR #213, Revision 2) went through two
different-provider fairness reviews (Antigravity): R1 had 12 findings, R2 had 5 new problems, and both were dispositioned in protocol §12.
A re-review of Revision 2 remains a pre-freeze gate. No trial has run. The Owner needs to choose:
1. **Providers and model IDs**, for example Claude Code with a named Claude model, and Codex with a named OpenAI model.
2. **H2 metric:** default strict win rate (wins / pairs, D16's plain wording); option A, shared credit for equal non-zero results;
   option B, wins / decisive pairs. Recommendation: **option A**, because it avoids penalizing HowlFrame when both arms are perfect, while
   mutual failures still earn nothing. Only the Owner can ratify a reading looser than the directive's wording.
3. **H4 runtime gate:** default Python, Go and JavaScript; option C, Python and JavaScript only, with Go reported.
   Recommendation: **default** (no relaxation), and report Go's ratio prominently either way.
4. **Spend cap** for a 4-run dry run, then the 96-run pilot (4 tasks × 4 languages × 2 providers × 3 repetitions), which is explicitly
   non-conclusive.

**Decision:** _(Owner)_
