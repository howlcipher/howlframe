# Final handoff: adoption, correctness and competitive-validation mission

Dates: 2026-10-21 to 2026-10-22. Implementing agent: Claude Opus 5.5 (Anthropic). Reviewers: Codex (OpenAI) and
Antigravity (Google, Gemini).
**Update 2026-10-22:** at the Owner's request, HowlFrame #211 and HowlBoard #20 are **merged**. HowlFrame #212 is **merged**
after Antigravity's round-4 review accepted its final code (`7de10d4a`). Later that day the Owner accepted the remaining
recommendations: branch protection is applied (B), #181 is closed with #216 and #217 opened (E), the Milestone 3 correction and R5
supersession are recorded (C, D), Go 1.25 is documented (A), `process:env` step 1 is implemented in its own PR (H), and the pilot
options are recorded (I). No model comparison trial was run, and none may run without a spend cap.
**Update 2026-10-23:** #219 (H step 1) is merged; the five gaps its independent review found are refused in a follow-up PR;
`verify_workflows.py` runs against current HowlBoard; H step 2 is deferred.

## Pull requests

| PR | Branch @ final head | What | Hosted CI | Independent review | Status |
|---|---|---|---|---|---|
| howlboard #20 | `feat/adopt-evidencedesk-ledger-validation` @ `770b42b` | `make import` validates with EvidenceDesk's ledger module (vendored, hash-pinned); HowlBoard's projection unchanged | Pass | Codex R1 **REJECT** (2 P2 contract gaps, fixed) → R2 **ACCEPT WITH NOTES** (note fixed); R2 confirmed by HTTP that the served data is the import | **Merged** 2026-10-22 as `24c5917` (decision F approved) |
| howlframe #211 | `feat/evidencedesk-ledger-parse-within` @ `9dbd8175` | `ledger.howl` exports `parse_within`; EvidenceDesk behavior unchanged | 4/4 pass | Covered by the #20 reviews (byte identity; 27-case EvidenceDesk probe identical) | **Merged** 2026-10-22 as `a154ddac` |
| howlframe #212 | `fix/hf05-value-position-stack-effects` @ `7de10d4a` | HF-05: the checker and HFIR adapter refuse value-less forms in value positions before any effect | 4/4 pass on the final head | Codex R1 filtered (no verdict; probes used), Codex R2 filtered; Antigravity R2 **REJECT** → fixed; Antigravity R3 **REJECT** → fixed in `7de10d4a`; Antigravity R4 **ACCEPT** at `7de10d4a` (`evidence/reviews/hf05_antigravity_round4.md`) | **Merged** 2026-10-22 as `6912b0bb` (decision G approved). Head `1e7ab8a6` adds only a `change_log.md` conflict resolution against `main`; the full gate passed on that merged tree and hosted CI 4/4 |
| howlframe #213 (draft) | `experiments/competitive-v1-protocol` @ `28febce3` | competitive-v1 preregistration, Revision 2; no trials | 4/4 pass | Antigravity fairness R1 **NEEDS REVISION** (12 findings) → Rev 1; R2 **NEEDS REVISION** (5 new) → Rev 2; R3 (2026-10-22) **READY FOR OWNER DECISION**, no blocking problem; its 3 non-blocking suggestions applied | Draft; stays unmerged until decision I |
| howlframe #214 | `docs/adoption-competitive-validation` | These reports and the evidence | — | — | Ready |

*Erratum (2026-10-25, finding F2 of `docs/competitive-readiness/HOWLFRAME_CURRENT_VERIFICATION.md`): #213's head at the time of this handoff was `1116c48b` (the Owner pilot-options record), not `28febce3`. The row above is left as originally written.*

Unrelated open PRs, not touched: #178, #206, #210.

## Required deliverables

| Document | Location |
|---|---|
| Owner decisions | `docs/adoption/HOWLFRAME_OWNER_DECISIONS.md` |
| Adoption report | `docs/adoption/HOWLFRAME_ADOPTION_REPORT.md` |
| Correctness follow-ups | `docs/adoption/HOWLFRAME_CORRECTNESS_FOLLOWUPS.md` |
| Competitive protocol | `experiments/competitive-v1/HOWLFRAME_COMPETITIVE_PROTOCOL.md` (PR #213), with `FAIRNESS_REVIEW_R1.md` and `R2.md` |
| Competitive results | **Not produced: no trial was approved or run** |
| Plan and baseline | `docs/adoption/EXECUTION_PLAN.md` |
| Evidence | `docs/adoption/evidence/` (harnesses, outputs, review reports and probe matrices) |

## Test results on the final heads

* **HowlBoard #20 @ `770b42b`:** `make test` 21/21 (pinned compiler `88e49597`), and earlier heads also against HowlFrame `main` and #211's
  compiler. `test_docs.py`, `test_seo.py`, `test_timeline_layout.py` (Chromium) and the CI import step pass. A clean-environment
  reproduction passed at `135cdf7`; later commits changed only docs and tests (the reviewer confirmed this). Hosted CI passes.
* **HowlFrame #211 @ `9dbd8175`:** new CLI test; EvidenceDesk `scripts/test.sh` passes against the historical Board (the only
  configuration where its R5 adapter step can run; decision D). Hosted CI 4/4.
* **HowlFrame #212 @ `7de10d4a`:** gofmt, `go build ./...`, `go vet ./...`, `go test ./...`, the write-cost, v2 and real-app benchmark
  proofs, the v2 harness unittest and `scripts/ci_acceptance.sh` (M1 + M2 frozen and regression suites) all pass, locally on the final tree.
  The corpus diff (9,845 local `.howl` programs) is one intended change. All 49 defect probes are refused and the 10 valid ones accepted.

## What each independent review established

* **HowlBoard adoption (Codex):** it is genuinely used. The real `make import` compiles the importer that `use`s the vendored module,
  and the compiled server serves exactly the imported missions and timeline. Projections are byte-identical to the old importer on
  every input both accept. No grant changed. The truth test was not weakened. Two contract gaps, both now fixed, were the
  undocumented 120-character ID limit and an overstated failure guarantee.
* **HF-05 (Codex probes plus Antigravity reviews):** each round found a new variant of one root cause (type joins hiding void
  paths). Every variant found is fixed and pinned by tests. The last fix generalizes the rule, but was verified only by the implementer.
* **Protocol fairness (Antigravity):** found bias in both directions, and statistical errors (pseudoreplication, an inverted power
  rule, mutual-failure credit). All are dispositioned. Where the protocol reads D16, the plain wording is now the default and any
  relaxation is an Owner option.

## Outstanding Owner decisions

A–H are decided (H step 2 deferred: it breaks TaskFlow's frozen test and no consumer needs it). I's spend cap is open.
Details are in `HOWLFRAME_OWNER_DECISIONS.md`.

## Post-merge check (2026-10-22)

On HowlBoard `main` @ `24c5917`, with a compiler built from HowlFrame `main` @ `a154ddac`: `make import` imports the sample
ledger (72 entries, 16 missions) and `make test` passes. The vendored module's SHA-256 matches HowlFrame `main`, and its cited
upstream commit `9dbd8175` is in `main` history.

## Recommended next action

Build the four pilot tasks, frozen tests and containers for competitive-v1 (no paid runs), then set a spend cap for the
4-run dry run.
