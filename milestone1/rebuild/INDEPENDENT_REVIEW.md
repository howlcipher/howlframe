# Independent review of fresh Milestone 1 candidates

Date: 2026-10-20. Reviewer: milestone3_requirements, distinct from the three
candidate implementers and root repair implementer, in the same Codex session.
This is role separation, not proof of independent providers or human authorship.

Decision at this review point: **PENDING**, with a reproducible ledgerlens exact
aggregate blocker still open. Historical acceptance is preserved, not reused as
fresh proof. No frozen requirements/tests were changed by this reviewer.

## Evidence and reproduction

Repository reviewed initially at HEAD 9210f20b plus the shared working tree.
Host Linux 7.2.4-ogc3.1.fc44.x86_64, x86_64, Go1.26.0, Python3.14.4.
Reviewer independently built `go build -o /tmp/m1-independent-hf .` then built
each `milestone1/rebuild/NN-name/src/name.howl` to
`/tmp/m1-independent-name.hfbc`. For each original application, ran:

```
HF_BIN=/tmp/m1-independent-hf APP_ARTIFACT=/tmp/m1-independent-NAME.hfbc \
python3 milestone1/NN-NAME/tests/acceptance/run_acceptance.py \
milestone1/NN-NAME/tests/acceptance/launcher.example.sh -v
```

Fresh full results: ledgerlens35/35, healthwatch29/29, taskflow32/32, logs in
review_evidence/*_frozen.txt. Compiler/artifacts run on host tools with loopback;
no OS sandbox or container isolation claimed. This review is not yet clean-HOME
reproduction of final selected canonical files or a final commit certificate.

## Held-out findings and repairs

1. **Ledgerlens R3/R7/R18, OPEN:** 92,234 valid rows each amount
   999999999999.99, default maxrows100000, expected total income
   92233999999999077.66. At frozen-suite4GiB accounting budget the VM legitimately
   hits LIMIT_EXCEEDED with empty stdout: not evidence of incorrect arithmetic.
   Raising only the accounting ceiling to1TiB (not reserved host RAM) reaches
   actual arithmetic and exits1 with raw RUNTIME_ERROR integer overflow. This is
   valid input, not a row-limit violation; exact sums and diagnostic hygiene do
   not permit an undisclosed int64 aggregate cap. See held_out.py, held_out.json,
   raised_budget_aggregate.txt. Requires repair before final acceptance.
2. **Taskflow R13, repaired targeted:** writable absent --history plus denied
   --report originally created history then returned77. Root added observational
   scoped grant checks and prechecked both paths before either write. Reviewer
   rebuilt compiler/artifact and confirmed exit77, no stdout, no history created.
   See repaired_denials_and_empty_state.json. Full repaired suite rerun underway.
3. **Healthwatch R7/R9, repaired targeted:** existing zero-byte state originally
   treated as absent: status exited0; run rewrote state then attempted network.
   HEAD historical source rejected correctly, so this was a replacement regression.
   Root distinguished missing from present-empty input. Reviewer rebuilt and both
   status/run now return65, state byte-unchanged, no report/request attempt.
   See health_empty_state.json and repaired_denials_and_empty_state.json.
4. Healthwatch unknown nested assertion keys rejected65 with appropriate path;
   taskflow recursive variable cycles rotate to earliest declaration and correct
   line. These independently selected probes pass.

## Matched baseline comparison and replacement recommendations

Built the HEAD historical .howl sources extracted through `git show HEAD:path`
using the same reviewer compiler. See baseline_comparison.json.

| Candidate | Demonstrated comparison | Recommendation |
|---|---|---|
| ledgerlens | Valid13-digit budget: old exits65, fresh returns exact canonical budget; explicit dictionary aggregate structure replaces composite sorting keys | Replace only after exact-aggregate blocker fixed and all tests/reproduction rerun |
| healthwatch | Fresh independently recreated monitoring workflow passes all29 frozen tests, but introduced empty-state regression absent historically; now targeted repaired | Reconsider after repaired full suite; no measured runtime superiority claimed |
| taskflow | Nested environment reference in dry-run: old exits77, fresh keeps literal and exits0; segmented lexer preserves argv instead of rescanning inserted text | Replace after repaired full suite and grant-query regression evidence |

Tradeoffs: new ledgerlens record dictionaries and general CSV state machine improve
inspectability and remove a fixed sort-key bound, but add allocations; fast path
avoids per-character processing for unquoted rows. Taskflow segmented words make
literal/reference boundaries explicit but allocate intermediate records; recursive
DAG/variable traversal still depends on finite call-depth/instruction grants.
Healthwatch validates in fixed order and retains denied-check resilience; a rewrite
can introduce edge regressions, as this review demonstrated. No comparative speed
or memory claim follows from source length or all-green supplied tests.

## D24 A-J evidence assessment

A: original requirements/acceptance remain frozen and unchanged relative to HEAD.
B: independently built standalone bytecode succeeds for all three; application
logic is HowlFrame, no generated application-language runtime helper.
C: original independent acceptance35/29/32 pass; held-out failures above retained.
D: fresh builds and execution verified, but final canonical clean-environment
reproduction still required after repairs/selection.
E: candidate journals record separate autonomous requirements-first implementation
and repairs; no human implementation fix observed. No historical evaluator
identity or cross-provider proof invented.
F: original user guides remain available; selected canonical paths must be updated
consistently and useful workflows independently rerun after selection.
G: regressions and limits must include these review findings, with failed evidence
retained and repaired probes rerun; exact-aggregate defect remains open.
H: frozen suites exercise scoped grant denials and resource limits. New
observational grant query needs root's regression suite; this is not OS isolation.
I: development journals/history retained, final selected source hashes/commit and
handoff pending.
J: **not accepted yet**; review remains open until the blocker and final evidence
items are satisfied. Do not convert all-green frozen tests into unconditional
acceptance.

## Final-source follow-up, 2026-10-20

Root repaired aggregate arithmetic with two nonnegative base1,000,000 limbs.
Each permitted per-row cents value is below1e14; at most1e7 rows keep the high
limb below1e15, safely within VM integer range. Income/expense accumulate
separately and net subtraction borrows the low limb without combining into an
int64. Category ordering and budget comparisons use exact pair/decimal logic.
No requirements or tests were weakened to permit a hidden aggregate cap.

Reviewer rebuilt the final compiler and all three candidates; full frozen suites
again pass35/29/32 (review_evidence/*_final_frozen.txt). Taskflow destination-denial
and healthwatch present-empty-state repairs independently pass as recorded above.
The original92234-positive-row workload now independently returns exact total
92233999999999077.66 with exit0, empty stderr,72.921 seconds under1TiB cumulative
VM allocation accounting (not reserved host RAM). Original failed output remains
preserved. Positive and mixed-negative final probe script/results are in
review_evidence/exact_aggregate_final.{py,json}; mixed-negative execution pending
at the time of this follow-up. No measured speedup claim follows from this test.

Replacement rationale remains strong for ledgerlens and taskflow correctness
improvements. Fresh healthwatch now matches the historical schema and empty-state
behavior in held-out comparison; no functional superiority has been demonstrated.
It is a separately recreated implementation satisfying the supplied contracts,
not an evidence-based runtime performance upgrade. Parent should explicitly select
or retain it on maintainability/rebuild grounds rather than imply proven superiority.

Final acceptance still awaits completed mixed-negative evidence and final clean
build/run of committed selected candidates plus relevant root core gates.

Mixed-negative final probe completed independently:92234 maximum negative rows
plus one maximum positive row, count92235, exact expense92233999999999077.66,
income999999999999.99, net-92232999999999077.67. Exit0, empty stderr,89.192 seconds.
Both aggregate cases pass; that arithmetic blocker is closed. Only final committed
clean reproduction/relevant core gates remain before a final D24 acceptance decision.
