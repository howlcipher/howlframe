# Fresh ledgerlens candidate, 2026-10-20

This candidate was written from the frozen requirements after the owner requested starting
all milestones again. Historical acceptance was not treated as proof. Earlier in this run,
the historical source was revalidated and its budget parsing corrected; that correction
is distinct from this independently written replacement candidate.

The replacement uses a CSV state machine, dictionaries containing transaction and
aggregate records, selection sorting for category expenses, and canonical decimal strings
for uncapped budgets. It does not copy the historical implementation. There are no
compiler/runtime changes and no frozen requirements, acceptance files, or fixtures changed.

Architecture comparison: typed record dictionaries make aggregate fields explicit and
selection sorting avoids a fabricated fixed maximum for sort keys. General CSV parsing is
simple to inspect, but slower on unquoted rows, so an equivalent delimiter-only fast path
was added for lines containing no quote. Arbitrary-length budget strings avoid bounded
integer conversion and retain exact output. Transaction values and aggregate sums still use
the VM's bounded integers; extreme permitted row counts and magnitudes can overflow the
aggregate. Full arbitrary-precision aggregation is not claimed.

Fresh evidence uses a compiler built from the working tree with `go build -o /tmp/m1-how .`,
Go 1.26.0 linux/amd64, Python 3.14.4, host Bash and Linux. Artifacts are built with:

```
/tmp/m1-how build milestone1/rebuild/01-ledgerlens/src/ledgerlens.howl -o /tmp/rebuild-ledgerlens.hfbc
HF_BIN=/tmp/m1-how APP_ARTIFACT=/tmp/rebuild-ledgerlens.hfbc python3 milestone1/01-ledgerlens/tests/acceptance/run_acceptance.py milestone1/01-ledgerlens/tests/acceptance/launcher.example.sh -v
```

Initial full candidate frozen suite: 35 tests, 35 passed. Subsequent fast-path and numeric
CLI normalization changes require the final rerun recorded below. The additional durable
`milestone1/tests/regression/fresh_boundaries.py` checks 13-digit and 100-digit budgets,
canonical output, zero budgets, strict over-budget comparison, and zero-padded row limits.
The old implementation reproduces the 13-digit-budget failure (65); the candidate succeeds.

The implementer authored this evidence. Independent final acceptance remains a separate
reviewer responsibility. No cross-provider identity or OS sandbox isolation is claimed.

Final candidate full frozen acceptance after unquoted-CSV fast path and numeric option
normalization: **35 tests, 35 passed, 0 failed**, wall 12.91 seconds for the whole suite.
Durable transcript: `evidence/2026-10-20_fresh_acceptance.txt`. Added boundary probes passed
against the fresh ledgerlens and taskflow artifacts (healthwatch still the baseline artifact
at this implementer's test point; its separate fresh candidate belongs to another agent).

## Independent review repair, 2026-10-21

The reviewer proved aggregate int64 overflow with 92,234 maximum-size positive
rows under a sufficient accounting budget. Root replaced aggregate integers with
base-1,000,000 high/low limbs; the entire allowed amount/row-count range fits
the high limb, while formatted combined cents can exceed int64. Independent
positive and mixed-negative exact sums now pass. Earlier limitation notes are
superseded by this repair; per-row amounts and resource budgets remain bounded.
