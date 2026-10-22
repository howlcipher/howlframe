# HowlBoard adoption of HowlFrame Milestone 3 work

Mission of 2026-10-21. HowlBoard base `f4c7bb5`; HowlFrame base `bfc4ff8f`. Implementing agent: Claude Opus 5.5 (Anthropic).
Reviewer: Codex (OpenAI). Evidence: `docs/adoption/evidence/` (harnesses in `evidence/harness/`; review reports in `evidence/reviews/`).

## 1. Status in one paragraph

HowlBoard's production ledger import (`make import`, `tools/ledger_import`) **would** adopt EvidenceDesk's ledger
validation module under HowlBoard PR #20 (with HowlFrame PR #211). The integration is **implemented** and
**independently verified**: a different-provider reviewer ran `make import` → `make run` → HTTP and found the served data
is the import. It is **not merged and not approved**, so HowlBoard `main` does **not** yet use any Milestone 3 code.
EvidenceDesk's *projector* was evaluated as a replacement and **rejected on evidence**. The incumbent HowlBoard projection
stays.

## 2. Starting point (verified)

* HowlBoard is already written entirely in HowlFrame, including its importer. The question was never "adopt the language".
  It was whether the Milestone 3 component should replace or improve a working HowlBoard component (directive D30 B).
* `main` had zero references to EvidenceDesk. The M3 consumption test had run against a stale checkout (`cd2f2fd`).
* On `main`, EvidenceDesk's own `verify_workflows.py` fails against current upstream HowlBoard, because its R5 adapter
  inserts a second `(grants ...)` form (`evidence/07_*`). It passes only against the stale checkout (`evidence/08_*`).

## 3. Incumbent vs EvidenceDesk projector (black-box, observable outputs)

| Property | HowlBoard importer (incumbent) | EvidenceDesk projector |
|---|---|---|
| HowlBoard's own truth contract (`TestLedgerImportTruth`) | Passes | **Fails 8 assertions**: a failed check's `exit_code` 7 becomes 0; `exit_code: 0` is invented where none was recorded; summaries are dropped; a check is fabricated for a non-verification event; `recording_mode`, `ledger_recorded_at` and the observed start are lost (`evidence/03`) |
| Mission fields on the 72-record sample | 7 more (`project`, `task_class`, `risk_level`, `recommended_executor`, override fields, `ledger_recorded_at`) | Missing (`evidence/02`) |
| Timeline timestamps | Kept | Zeroed for every event |
| `human_approval` / `human_rejection` / `implementation_recovered` | Projected | Ignored |
| Input ceiling | Limited by the VM memory charge: about 2,500 records at the default 64 MiB; 5,000+ at 256 MiB | 256 KiB, about 286 real records |
| Malformed / wrong-schema / duplicate / empty input | **Silently skipped, double-counted, or (empty file) the timeline wiped** | Refused with the line number before any write |
| Entry limit | Imports `MAX + 1`, then silently drops the rest | Exact; over-limit fails |
| Entry without `metadata` (schema-valid) | **Crashes** with raw `TYPE_ERROR` | Accepted |

Conclusion: replacing the importer with the projector would regress HowlBoard's core purpose ("should I believe this happened?").
The projector's real strength is its **input validation**, which is exactly where the incumbent was weakest.

## 4. Integration architecture

Options evaluated:

| Option | Pros | Cons | Decision |
|---|---|---|---|
| 1. Replace the importer with the projector | Smallest diff in HowlBoard | Every regression in §3 | Rejected |
| 2. **Internal component: vendor EvidenceDesk's `ledger.howl` byte-identical, call it before HowlBoard's projection** | Keeps HowlBoard's verified projection; gains validation; one file, hash-pinned; no new runtime or grant | Vendored copy can drift (mitigated by a hash test); needs a caller-chosen byte ceiling | **Chosen** |
| 3. Independently versioned dependency (HowlBoard builds the module from the pinned HowlFrame checkout) | No copy | Couples HowlBoard's build to a milestone demo directory inside the compiler repository; HowlFrame has no package mechanism | Rejected |

The one HowlFrame change (PR #211) adds `parse_within(raw, maximum, max_bytes)`. `parse` delegates to it with the frozen
262144-byte ceiling, so EvidenceDesk's behavior is unchanged: its full suite passes, and a 14-case projector comparison
plus the reviewer's 27-case probe are identical.

## 5. Changed workflow (HowlBoard #20)

`make import LEDGER=... MAX=...` keeps its interface and grants (`filesystem,database`). On every valid input tested, both
stores are **identical** to the incumbent's: the golden file was produced by the incumbent on the sanitized sample, and
7 more differential cases plus the reviewer's independent set (CRLF, Unicode IDs, a 2 MiB line, 2,000 entries on one task)
also match.

Behavior changes, all documented in the README and change log:

| Input | Before | After |
|---|---|---|
| Malformed JSON, truncated last line, non-object, wrong schema, missing or empty IDs | Skipped silently, or a raw crash | Refused, line N, exit 3, stores byte-identical |
| Duplicate `entry_id` | Counted twice | Refused |
| Empty or blank file | Timeline replaced with an empty one | Refused |
| More lines than `MAX` | `MAX + 1` imported, rest dropped | Refused at line `MAX + 1` |
| `MAX` of 0, negative or non-numeric | 1 / 0 imported / raw error | Usage, exit 2 |
| Over 3 MiB | Raw memory error past about 2.5 MB | Refused with the ceiling |
| `entry_id`/`task_id`/`agent_id` over 120 characters | Accepted | **Refused.** This is the vendored validator's limit. HowlPlane's schema sets no maximum; the longest real `task_id` in 443,371 records is 41 characters. Found by the reviewer |
| Entry without `metadata`, or non-object `metadata` | Raw `TYPE_ERROR` | Imported |

* `make import` raises the VM's cumulative memory **charge** to 256 MiB. This is a resource budget, not a grant (the reviewer agreed).
  Validation roughly doubled the charge, and the default 2,000-entry slice no longer fit in 64 MiB.
* Unchanged and now documented: the no-write guarantee covers **validation** failures. A write failure afterwards (a corrupt
  existing timeline, or a denied write) can leave missions updated with the old timeline, as before.
* Compatibility with real data: a local 443,371-record HowlPlane ledger has **zero** lines this validation refuses
  (aggregate counts only; no ledger content entered any repository, test or report).

## 6. Measurable value over the previous implementation

1. **No silent evidence loss.** Seven input classes that were silently dropped, duplicated, truncated or wiped are now refused
   with a line number, and the existing stores stay intact (14 rejection cases, each checked byte-for-byte).
2. **Two crash defects fixed:** a schema-valid entry without `metadata`, and non-object `metadata`.
3. **The documented workflow works:** the previous README command (`head -n 5000`, `MAX=5000`) failed with a raw memory error
   at typical record sizes. The documented command now succeeds, and oversize input fails with a clean message.
4. **Same output on valid input.** Projection semantics, ordering and verification truth are unchanged (golden file plus truth test).
5. Costs, stated against the incumbent (`evidence/05_sizes.txt`):
   * About 1.5× import time: 1,000 records 1.6–1.7 s vs 1.0–1.1 s; 2,000 records 5.5 s vs 3.6–3.7 s.
   * A larger memory charge: the new importer needs the 256 MiB budget for 2,000 records; the old one fit in 64 MiB.
   * **A lower maximum slice at equal budgets.** Given the same 256 MiB flag, the old importer imports 5,000 records
     (4.3 MB, 20 s). The new one refuses anything over its 3 MiB ceiling (about 3,500 records, 15.5 s). Against the previously
     *shipped* command (no memory flag), the new ceiling is higher: the old importer failed somewhere between 2,000 and 3,000
     records with a raw error. The 3 MiB ceiling was chosen to fit the 256 MiB charge with margin. Raising both is possible,
     but would raise the charge further. This trade-off is part of Owner decision F.

## 7. Verification

| Check | Result |
|---|---|
| HowlBoard `make test` (20 → 21 tests), pinned compiler `88e49597`, HowlFrame `main` and #211 | Pass |
| HowlBoard CI on #20 | Pass |
| Clean environment (fresh clones, empty HOME and Go caches, compiler from GitHub at the pin) | `make test`, `make import` pass |
| Mutation: old importer swapped in | 14 rejection cases and the limit/metadata test fail; preservation tests pass |
| Mutation: one-byte edit to the vendored module | Drift test fails |
| Denied capabilities (`database` only, `filesystem` only) | Exit non-zero, no stores written |
| SIGKILL storm, 40 rounds each, over a prior import | 0 torn files in both; partial missions-updated states pre-exist (old 27/40, new 15/40) |
| **Independent review, Codex round 1** | **REJECT**: 2 P2 contract gaps (undocumented ID limit; overstated failure guarantee). Everything else confirmed. HTTP not observable (sandbox) |
| **Independent review, Codex round 2** | **ACCEPT WITH NOTES**: both gaps resolved; `make import` → `make run` → HTTP shows the served data is the import; correction diff is documentation and tests only. Note (missing `agent_id` row) fixed in `770b42b` |

## 8. Adoption status, precisely

| Claim | Status |
|---|---|
| A real HowlBoard workflow invokes HowlFrame Milestone 3 code | **Implemented** on the branch; **independently verified**; **not merged** |
| HowlBoard `main` uses it | **No**, not until #20 and #211 merge |
| EvidenceDesk's projector replaces HowlBoard's importer | **No**, rejected on evidence |
| The previous importer is removed or deprecated | Its input path is replaced in place; the projection is kept on purpose |
| Owner approval of the behavior changes | **Pending** (decision F) |

Recommended merge order if approved: HowlFrame #211, then HowlBoard #20, after updating `VENDORED.md` to the merge commit if
#211 is squashed (the file hash is unchanged either way).
