# Final handoff: post-audit stabilization

## Status: **ready for Owner review; NOT merged.** No merge, force-push, history rewrite or push to `main` occurred.

| Item | Value |
|---|---|
| Branch | `fix/post-audit-stabilization` |
| Base | `5d2983b2c2311daf9b7494311ad719ff5dc51c09` (== `origin/main` at start and at last fetch) |
| Source-verified commit | `3832d83e7a4d8e33648db5c93d9611f4efde8441` (all local verification), plus `e472a928` (test-only fix after the first hosted CI run) |
| Head at handoff | `e472a928` plus this documentation commit (docs only) |
| Pull request | https://github.com/howlcipher/howlframe/pull/209 (open, mergeable) |
| Hosted CI | `e472a928`, run 39681079671: **build, acceptance, race, latest-go all pass**. The first run (`7823c901`, run 39680037886) had `build` **fail**: see below |
| Size | 34 non-docs files changed, about 2,200 insertions, 76 deletions; 12 commits |

## Gates (mission Phase 8)

| Gate | Result | Status of evidence |
|---|---|---|
| Filesystem containment | No escape in bounded concurrent tests; root-relative enforcement | Implemented; tested (red on baseline, 15 repeats, `-race`); **independently verified** (reviewer probes re-run by me) |
| Process containment | No survivors on normal exit, deadline, output-limit | Implemented; tested; independently verified. **Limit:** a program that deliberately kills its own supervisor can leave detached descendants (documented; not fixed). Linux only; other platforms weaker (documented) |
| TicketDesk restart | 1,000 tickets + 100 tokens under the documented command (0.3 s; 6,000 verified) | Implemented; tested; independently verified |
| Slow-reader isolation | Non-reading client cannot stall unrelated requests | Implemented; tested; independently verified |
| Assurance regression | Port number restored; m10 `5 tests, 0 failed, 1 expected failure` | Implemented; tested; the one XFAIL (m12 lost-race, Owner-waived) is **unchanged and still deferred** |
| Milestone 3 evidence | Current-upstream comparison; history preserved; no fabricated approval | Documented; claims independently checked against the HowlBoard repo |
| Frozen application tests | M1 35/29/32; M2 101, DX 13, reassessment 11, historical 10 | All pass; frozen files byte-unchanged |
| CI | Accepted suites, race, minimum Go, latest Go run on PRs and main | Implemented; passed on GitHub on `e472a928` |
| Independent review | Round 1 (Codex): REJECT, 3 defects. Round 2 (Antigravity `agy`): ACCEPT | See caveats below |
| Git safety | Nothing merged | Confirmed |

**Not complete / uncertain:** EvidenceDesk's browser workflow is not in CI (needs Playwright and a Board checkout; manual, passed locally on the final tree). Windows and macOS runtime behavior of the new code was cross-compiled, never executed. Power-loss durability is untested (as in the audit). Production readiness is not claimed.

## What happened that you should know
1. **My own regressions, caught late.** Single-file grants broke the projector (found by re-running the Board consumer check); the `go 1.25` bump broke go-target test scaffolding (found by the full race run); the first single-file fix reached a sibling through a symlink and an exec protocol bug killed commands that close their output (both found by the Round 1 reviewer). All were reproduced, tested and fixed.
2. **The first hosted `build` failed** on a test I wrote (see journal); fixed and re-run green.
3. **Reviewer provenance.** Codex hit its usage limit after round 1, so round 2 used `agy`. Its report contains wrong function and test names; I verified its substance by running its probe scripts myself. Treat round 2 as a genuine independent check, but not as a flawless document. No human reviewed the code.
4. The final verification logs predate the last two commits only in that those commits changed one test file and documents; the four hosted jobs ran on the final source.

## Files and evidence
- Plan, journal, security, results, reconciliation: `docs/remediation/HOWLFRAME_{REMEDIATION_PLAN,REMEDIATION_JOURNAL,SECURITY_REMEDIATION,REGRESSION_RESULTS,MILESTONE_RECONCILIATION}.md`
- Logs: `docs/remediation/evidence/` (baseline red runs, final runs, review reports, probes)
- Source: `internal/capability/scopedfs.go`, `internal/vm/{vm,http_response,exec_contain_*}.go`, `howlframe.go`, `go.mod`, `.github/workflows/`, `scripts/ci_acceptance.sh`, tests listed in the results document.
- The six audit documents were not modified.

## Owner decisions required
1. Milestone 3 / D30 B verdict wording, and approve, reject or retire the R5 addendum (no recorded Owner approval): `milestone3/EVIDENCE_STATEMENT_2026-10-21.md` §5.
2. Accept `go 1.25` as the minimum toolchain (required by the filesystem fix; Go 1.21+ users auto-fetch it).
3. Branch protection requires only `build`; consider adding `acceptance` and `race` (I cannot change protections).
4. Whether the supervisor-kill limit is acceptable, or whether to fund the runner-as-second-subreaper design.

## Follow-up register (not done here; none is a gate of this release)
| Priority | Item | Note |
|---|---|---|
| High | Supervisor-kill containment gap | Needs runner-level subreaper plus a registry of runner children |
| Medium | HF-05 checker/HFIR stack-effect gap | Accepts `(try_let (r (write_file ...)))`, fails at runtime after the effect |
| Medium | HF-12 raw runtime JSON on limit stops | Conflicts with frozen hygiene requirements; also the m12 XFAIL |
| Medium | HF-11 / #182 env allowlist | Deny-list gaps |
| Low | HF-13 file modes 0644 | Stores and outputs |
| Low | HF-14 live-memory bound | Budget is cumulative |
| Low | HF-20 loopback default | Servers bind all interfaces by default |
| Low | #147 fixed port in app tests | Latent flake |
| Low | Windows `GOOS=windows go build ./...` | `syscall.Flock` in `internal/testutil` (pre-existing) |
| Later | Cross-provider/human verification of the milestone applications; matched competitive trials; power-loss testing; HowlBoard adoption decision | Audit roadmap |

## Recommended next action
Owner reviews PR #209. If the decisions above are acceptable, merge it, then re-run `scripts/ci_acceptance.sh` and the EvidenceDesk workflow once on `main`. Stop there; do not start another remediation cycle without a new decision.
