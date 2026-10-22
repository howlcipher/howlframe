# Adoption and competitive validation: baseline and execution plan

Mission: move HowlFrame from demonstrations toward genuine adoption and falsifiable competitive evidence.
Started 2026-10-21. Implementing agent: Claude Opus 5.5 (Anthropic), in worktrees under `dev/worktrees/adopt-*`.

## Phase 0 baseline (verified live, 2026-10-21)

| Item | Observed |
|---|---|
| HowlFrame `origin/main` | `bfc4ff8f` (PR #209 merge, the stabilization). No commits since. CI on it: build, acceptance, race, latest-go all green |
| HowlBoard `origin/main` | `f4c7bb54` (PR #19). CI green. No open PRs or issues |
| HowlFrame open PRs | #178 (live-v1 harness, draft), #206 (M3 receipt-verifier spec freeze, draft), #210 (HFBC writer-format reporting, draft, from a separate Codex mission; not touched here) |
| HowlFrame open issues | #147, #157, #164, #174, #175, #181, #182, #194 |
| Branch protection on `main` | Required status checks: `build` only; `strict: false`; admins not enforced |
| Main checkouts | Both `dev/howlframe` and `dev/howlboard` are behind origin and in use by a concurrent Codex session; not touched |

### Status of named findings at `bfc4ff8f`

| Finding | Status | Basis |
|---|---|---|
| HF-01 filesystem containment | Fixed by #209 (root-relative `os.Root` I/O); independently verified by the #209 round-2 reviewer | `docs/remediation/HOWLFRAME_SECURITY_REMEDIATION.md` |
| HF-02 process containment (#181) | Fixed for ordinary abandoned descendants (Linux supervisor). **Not** fixed: a program that deliberately kills its own supervisor | Same; #181 is still open |
| HF-03 TicketDesk restart | Fixed; 1,000 tickets + 100 tokens in CI | `HOWLFRAME_REGRESSION_RESULTS.md` |
| HF-04 slow reader | Fixed; loopback regression in CI | Same |
| HF-08 Assurance regression | m10 port diagnostic restored; m12 lost-race stays an Owner-waived XFAIL | Same |
| HF-09 CI coverage | `acceptance`, `race`, `latest-go` jobs now run on PRs and main; only `build` is *required* | CI run on `bfc4ff8f`; branch protection API |
| HF-05 checker/HFIR stack effect | **Open. Reproduced on `bfc4ff8f`**: `(try_let (r (write_file ...)) ...)` passes `check` and `-compile-bc`, writes the file, then fails `STACK_UNDERFLOW` | This mission, `evidence/hf05_repro_baseline.txt` |
| HF-11 / #182 env forwarding | **Open.** Deny-list (`capability.CodeLoadingEnvNames`); audit-listed gaps still forwarded | `internal/capability/process.go` |
| M3 EvidenceDesk / HowlBoard adoption | Compatibility demonstrated against current upstream; **no adoption**: HowlBoard has zero references to EvidenceDesk | `milestone3/EVIDENCE_STATEMENT_2026-10-21.md`, rechecked here |

## Workstreams and acceptance criteria

| # | Workstream | Deliverable | Acceptance | Bound |
|---|---|---|---|---|
| W1 | Owner decisions A–E | `HOWLFRAME_OWNER_DECISIONS.md` | Each decision has evidence and a recommendation; approvals recorded only if actually given | Docs only; no protection changes |
| W2 | HowlBoard adoption | HowlBoard PR + `HOWLFRAME_ADOPTION_REPORT.md` | Real `make import`/server path runs the adopted code; incumbent truth contract still passes; old-vs-new differential retained; different-provider review | One HowlBoard PR plus, if needed, one minimal HowlFrame app-code PR |
| W3 | HF-05 | HowlFrame PR + `HOWLFRAME_CORRECTNESS_FOLLOWUPS.md` | Invalid value-position uses rejected by `check`/build before any effect; effect-only statements still valid; HFIR path consistent; compiler/VM suites green; different-provider review | Checker/validator only; no HFIR redesign |
| W4 | HF-11 / #182 | Policy proposal in `HOWLFRAME_CORRECTNESS_FOLLOWUPS.md` | Consumers and compatibility impact measured; no default change without agreement | Proposal only unless authorized |
| W5 | Competitive protocol | `experiments/competitive-v1/` preregistration + `HOWLFRAME_COMPETITIVE_PROTOCOL.md` | Language-neutral tasks, frozen tests, held-out set, provider controls, scoring and stopping rules; fairness review before any trial | No paid trials without approved budget |
| W6 | Handoff | `HOWLFRAME_FINAL_HANDOFF.md` | Every claim labelled implemented / independently verified / approved / merged / deferred | — |

Stop conditions are the mission's Phase 7. Nothing is merged; branch protection is not changed.
