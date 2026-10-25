# Independent Fairness Review: HowlFrame Competitive Protocol (`competitive-v1`, Revision 2)

**Review Date:** 2026-10-22  
**Reviewer:** Independent Reviewer (Antigravity / Gemini)  
**Target Documents:** [`HOWLFRAME_COMPETITIVE_PROTOCOL.md`](./HOWLFRAME_COMPETITIVE_PROTOCOL.md), [`parameters.json`](./parameters.json)  
**Prior Reviews:** [`FAIRNESS_REVIEW_R1.md`](./FAIRNESS_REVIEW_R1.md), [`FAIRNESS_REVIEW_R2.md`](./FAIRNESS_REVIEW_R2.md)  
**Reference Directive:** [`context/DIRECTIVE_section9_competitive_targets.md`](./context/DIRECTIVE_section9_competitive_targets.md)

---

## 1. Disposition of Prior Findings (R1 Findings 5, 7, 8 and R2 Problems 1–5)

### Partially Resolved R1 Findings

1. **Finding 5 (Correctness Scoring): RESOLVED.**  
   §7.2 explicitly separates non-zero shared successes from 0/0 mutual failures and mandates that mutual failures earn zero credit under all metrics, eliminating artificial score inflation on mutual failures while preserving symmetric evaluation.

2. **Finding 7 (Runtime Target): RESOLVED.**  
   §1 Table 1, §7.4, and §7.8 establish all three baselines (Python, Go, JavaScript) as the plain-reading default runtime gate in strict adherence to Directive D16, while neutrally and clearly framing the exclusion of Go as Owner Option C.

3. **Finding 8 (Statistics & Sizing): RESOLVED.**  
   §3.2(1), §7.7, and §9 align task pool sizing to full-study requirements, adopt task-clustered modeling to handle run-level correlation, and size the study for joint 80% power across all three baselines under realistic effect sizes.

### New Problems Introduced in Revision 1 (R2 Problems 1–5)

4. **R2 Problem 1 (Candidate Pool Sizing Contradiction): RESOLVED.**  
   §3.2(1) decouples the 4-task pilot pool (≥ 6 candidates each for D1–D4) from the full-study pool, requiring the post-pilot full-study pool to hold at least 1.5× the required independent tasks (e.g., ≥ 68–120 candidates for 45–80 tasks).

5. **R2 Problem 2 (Statistical Power Inversion and Multiplicity in Sizing): RESOLVED.**  
   §7.7 and §9 replace the fragile point-estimate refutation threshold with a three-outcome decision rule ("Met", "Not met", "Inconclusive") and power sizing for joint 0.80 probability across all three baselines (0.928 per baseline) at Bonferroni-adjusted $\alpha = 0.05/3$ for true win rates $p_1 \in [0.65, 0.75]$.

6. **R2 Problem 3 (Mutual Failure 0/0 Inflation in Primary Score): RESOLVED.**  
   §7.2 defines mutual failures as both arms scoring 0 held-out pass fraction and explicitly excludes them from earning credit under the default strict win rate, Option A, and Option B.

7. **R2 Problem 4 (Semantic Divergence from Directive D16 "Matched Correctness Wins"): RESOLVED.**  
   §7.2 and §7.8 set the strict win rate ($W/N$) as the plain-reading default, while Owner Options A (shared credit $(W + 0.5 T_{\text{shared}})/N$) and B (decisive rate $W/(W+L)$) are clearly, neutrally, and fairly defined for pre-freeze Owner selection.

8. **R2 Problem 5 (Unilateral Exemption of Go from Runtime Gate H4): RESOLVED.**  
   §1 Table 1, §7.4, and §7.8 retain Python, Go, and JavaScript in the default H4 gate, while fairly and clearly presenting the compiled-vs-interpreted VM distinction as Owner Option C for explicit pre-freeze ratification.

### Evaluation of Owner Options Clarity and Fairness (H2 Options A/B, H4 Option C)
- **H2 Options A & B (§7.2, §7.8):** The default (strict win rate $W/N$) follows the plain wording of Directive D16. Option A (shared credit on non-zero successes) and Option B (decisive win rate excluding ties and mutual failures) are statistically standard, accurately defined, transparently contrasted, and prevent hidden score inflation.
- **H4 Option C (§7.4, §7.8):** The default (gating against Python, Go, and JavaScript) strictly respects Directive D15/D16. Option C (gating on Python and JavaScript, reporting Go as characterization) is clearly articulated as an architectural distinction (interpreted VM vs. compiled system binary) without pre-empting the Owner's authority.

---

## 2. New Blocking Problems in Revision 2

**None.** Revision 2 introduces no new blocking problems.

- **Task-Pool Sizing (§3.2.1, §9):** Sizing is consistent and achievable. The pilot authors ≥ 24 candidates across D1–D4 for 4 tasks; the full study authors a fresh pool of ≥ 1.5× the determined task count (e.g., ≥ 68–120 candidates for 45–80 tasks) after the pilot.
- **Power Derivation (§9):** Derivation correctly incorporates joint multiplicity ($0.80^{1/3} \approx 0.928$), Bonferroni correction ($\alpha = 0.05/3$), and cluster design effects ($1 + 5\rho$), yielding a viable planning range of 45–80 independent tasks to be finalized from the pilot's empirical intra-task correlation $\rho$.
- **Three-Outcome Decision Rule (§7.7):** Preregistering "Met", "Not met", and "Inconclusive" using task-clustered intervals eliminates sampling-noise false refutations at the 60% boundary while faithfully preserving the directive targets.
- **Consistency with `parameters.json`:** All operational limits, random draw beacon specs, pilot configurations, full-study task ranges [45, 80], defaults, and Owner options match between [`HOWLFRAME_COMPETITIVE_PROTOCOL.md`](./HOWLFRAME_COMPETITIVE_PROTOCOL.md) and [`parameters.json`](./parameters.json).

---

## 3. Non-Blocking Suggestions

1. **Table 1 Refutation Column Alignment (§1 vs. §7.7):** In §1 Table 1, the "Refuted if" column for H1, H3, H4, and H5 still lists bare point-estimate inequalities (e.g., `< 80%`, `Ratio > 0.8`), whereas H2 cites "§7.7 outcome *not met* against any one baseline". For complete internal consistency, Table 1's refutation column for H1, H3, H4, and H5 should also reference the §7.7 "not met" outcome.
2. **`parameters.json` Metric Labeling (`h2_reported`):** In `parameters.json` line 59, `"h2_reported"` lists `["W/T/L", "wins/pairs", "wins/(wins+losses)"]`. Updating `"W/T/L"` to `"W / shared successes / mutual failures / losses"` and adding `"(wins + 0.5*shared successes)/pairs"` would exactly mirror the reporting obligations in §7.2.
3. **Illustrative Example Text (§3.2.1):** §3.2(1) cites "(for example, ≥ 135 candidates for 90 tasks)" as an illustrative 1.5× calculation from Revision 1's 60–90 task range. Updating the text to "(for example, ≥ 120 candidates for 80 tasks)" will match the revised 45–80 task planning range in §9.

---

## 4. Overall Verdict

**READY FOR OWNER DECISION**

Revision 2 successfully resolves all remaining blocking and partially resolved issues from R1 and R2. The protocol is statistically rigorous, internally consistent, methodologically protected against bias and tampering, and strictly adheres to Directive D14–D16 while properly framing open architectural interpretations (H2 metric selection, H4 Go gating) for pre-freeze Owner determination.
