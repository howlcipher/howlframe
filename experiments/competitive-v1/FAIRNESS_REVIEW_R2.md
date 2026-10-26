# Independent Fairness Review: HowlFrame Competitive Protocol (`competitive-v1`, Revision 1)

**Review Date:** 2026-10-21  
**Reviewer:** Independent Reviewer (Antigravity / Gemini)  
**Target Documents:** [`HOWLFRAME_COMPETITIVE_PROTOCOL.md`](./HOWLFRAME_COMPETITIVE_PROTOCOL.md), [`parameters.json`](./parameters.json)  
**Prior Review:** [`FAIRNESS_REVIEW_R1.md`](./FAIRNESS_REVIEW_R1.md)  
**Reference Directive:** [`context/DIRECTIVE_section9_competitive_targets.md`](./DIRECTIVE_section9_competitive_targets.md)

---

## 1. Disposition of the 12 R1 Findings

1. **Finding 1 (Prior Evidence): RESOLVED.**  
   §2 accurately reports `heldout-v0` as 0/1/29 over expressible tasks (0/1/31 overall) and clarifies that Benchmark v2 Track B is unautomated.

2. **Finding 2 (Bias Guards): RESOLVED.**  
   §3.2(3) and §8 replace the per-domain cap with a global benchmark cap of at most 1 of 4 pilot tasks (25%) and at most 15% of full-study tasks across all domains.

3. **Finding 3 (Task Steering & Contamination): RESOLVED.**  
   §3.2(1) bars reusing any internal HowlFrame task cards with mandatory de novo authoring, and §3.2(4) eliminates seed-mining by deriving the draw seed from a future NIST Randomness Beacon pulse.

4. **Finding 4 (Feasibility Screen): RESOLVED.**  
   §3.2(2) restricts the feasibility screen solely to baseline stdlibs, and §3.2(5) retains tasks HowlFrame cannot express and scores them as failures in the primary analysis.

5. **Finding 5 (Correctness Scoring): PARTIALLY RESOLVED.**  
   §7.2 symmetrizes the acceptance pass filter and prevents ceiling penalties on 100%/100% ties via $(W + ½T)/N$, but contradicts R1 by awarding 0.5 tie credit to 0/0 mutual failures.

6. **Finding 6 (Speed Metric): RESOLVED.**  
   §7.3 replaces the distorted 45-minute imputed median ratio with Restricted Mean Survival Time (RMST) over 45 minutes and reports concordant-pair medians as secondary.

7. **Finding 7 (Runtime Target): PARTIALLY RESOLVED.**  
   §7.4 removes the unfair "fastest baseline" Go hurdle by gating against Python and JavaScript, but unblocks Go only by demoting it to reported-only pending Owner confirmation (§7.4, §12) rather than benchmarking against an agreed Go workload.

8. **Finding 8 (Statistics & Sizing): PARTIALLY RESOLVED.**  
   §9 resolves pseudoreplication by adopting task-clustered modeling and sizing by independent tasks, but derives a 60–90 task range that is mathematically incompatible with §3.2(1)'s pool size and relies on an inverted hypothesis test.

9. **Finding 9 (Documentation Asymmetry): RESOLVED.**  
   §4 equips baseline containers with offline stdlib references, establishes a pre-freeze documentation audit gate, and §6 isolates documentation-reading tokens.

10. **Finding 10 (Test Tampering): RESOLVED.**  
    §3.3 and §4 mount acceptance tests as read-only outside the workspace, enforce fresh container scoring with hash-verified tests, and report modifications as tampering.

11. **Finding 11 (Execution Mechanics): RESOLVED.**  
    §5 introduces a programmatic `task-complete` handshake command, mandates co-scheduled baseline runs, and logs provider wait time to report active compute time.

12. **Finding 12 (Directive Completeness): RESOLVED.**  
    §7.5 adds baseline maintenance implementations with matched comparisons, and §7.6 replaces the conjunctive 10-test gate with a pooled consistency rule.

---

## 2. New Blocking Problems Introduced in Revision 1

1. **Candidate Pool Sizing Contradiction (§3.2.1 vs. §9):**  
   §3.2(1) specifies authoring "≥ 6 new candidates per domain." Across the 5 domains in §3.1, this yields only ≥ 30 candidate tasks. Yet §9 specifies a full-study draw of 60–90 independent tasks. It is mathematically impossible to draw 60–90 unique tasks from a pool of 30. §3.2(1) must be revised to mandate authoring a pool large enough for the full study (e.g., ≥ 18–25 candidates per domain, or ≥ 100 total candidates), or explicitly decouple the pool requirements between the 4-task pilot and the full study.

2. **Statistical Power Inversion and Multiplicity in Sizing (§9, §1 H2):**  
   §9 derives the sample size of 153 pairs based on testing $H_0: p \le 0.50$ vs. $H_1: p = 0.60$ with power 0.80. This tests whether HowlFrame is superior to parity ($p > 0.50$), NOT whether HowlFrame satisfies the Directive target of $p \ge 0.60$. Because H2 refutes if the point estimate is $< 60\%$ (§1 Table 1), a true win rate of 60% yields an observed estimate $< 60\%$ approximately 50% of the time, resulting in an unacceptably high ~50% Type II error (false refutation) rate at the target. Sizing must be powered to detect that performance meets or exceeds 0.60 against a refutation threshold under an assumed true effect $p_1 > 0.60$ (or non-inferiority bound). Additionally, the derivation sizes a single pairwise comparison, ignoring the multiplicity penalty of testing against three baselines jointly ($0.8^3 \approx 0.51$).

3. **Mutual Failure (0/0) Inflation in Primary Score (§7.2):**  
   §7.2 defines ties as equal held-out fractions "including both at 1.0 and both at 0" and awards 0.5 points per tie: `(wins + ½ ties) / pairs`. Crediting HowlFrame with half a win when BOTH arms fail (0% held-out pass rate) creates perverse scoring incentives: on difficult, buggy, or out-of-spec tasks where neither model produces working code, HowlFrame is awarded 0.5 points toward its 60% target. This artificially pulls HowlFrame's score toward 50% on catastrophic failures. Mutual failures (0/0) must score 0 for both arms (or be excluded from decisive pair scoring).

4. **Semantic Divergence: $(W + ½T)/N$ vs. Directive D16 "60% Matched Correctness Wins" (§1 H2, §7.2):**  
   Directive D16 explicitly specifies: "Matched correctness wins: At least 60% against each of Python, Go, and JavaScript." Revision 1 replaces strict wins with the Borda/Mann-Whitney common language effect size `(wins + ½ ties) / pairs`. Under this rule, HowlFrame could pass H2 with only 40% actual wins if 40% of pairs tie and 20% lose ($0.40 + 0.5 \times 0.40 = 0.60$). While $(W + ½T)/N$ properly avoids ceiling penalties on mutual successes (1.0/1.0), calling it "60% matched correctness wins" diverges from the plain wording of Directive D16. If the Owner requires strict wins ($W/N \ge 60\%$) or decisive wins ($W/(W+L) \ge 60\%$), using $(W + ½T)/N$ as the sole gating criterion without explicit Owner ratification represents an unapproved lowering of the directive's standard.

5. **Unilateral Exemption of Go from Runtime Gate H4 (§1 H4, §7.4):**  
   Directive D15 mandates comparisons against "Python, Go, and JavaScript", and Directive D16 mandates "Execution time within 2×". Revision 1 restricts the H4 refutation gate solely to Python and JavaScript, demoting Go to "reported but is not a gate." While R1 correctly noted that comparing against the "fastest baseline" (min runtime) was an unfair tightening, completely exempting Go from the refutation gate alters the core scope of D16. Although §7.4 flags this as an interpretation requiring Owner confirmation (decision I), until the Owner formally ratifies this exemption, H4 remains an unratified modification of the directive.

---

## 3. Overall Verdict

**NEEDS REVISION**

Revision 1 substantially improves protocol rigor, resolving 9 of 12 original findings and establishing sound foundations for container isolation, beacon seeding, and test integrity. However, it cannot be certified as READY FOR OWNER DECISION due to:
- An internal contradiction between task pool generation (§3.2.1) and full-study sizing (§9).
- Distortion in §7.2 awarding 0.5 win credits to mutual 0/0 failures.
- An inverted power calculation in §9 yielding ~50% false refutation at the directive target.
- Unratified modifications to Directive D16 targets (H2 effect size vs. strict wins; exclusion of Go from H4).

A concise Revision 2 fixing the pool sizing, 0/0 failure crediting, and statistical power derivation, while formally structuring the H2 metric and H4 Go gate as explicit Owner decision options, will achieve complete readiness.
