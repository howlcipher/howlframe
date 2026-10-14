# ACCEPTANCE: ledgerlens

> **FROZEN, dated 2026-10-13.** Companion of `REQUIREMENTS.md`; changes require a separately approved revision (REQUIREMENTS.md 1.5).

## How acceptance is decided

1. Build the implementation from `src/` into `build/ledgerlens.hfbc` with `howlframe build` in a clean environment (no pre-existing artifacts, caches or generated code), and capture the commands and output as evidence (D24-D).
2. Provide a launcher that satisfies REQUIREMENTS.md 1.2 (the reference one is `tests/acceptance/launcher.example.sh`; set `HF_BIN` and `APP_ARTIFACT`).
3. Run the independent suite from a scratch directory outside the source tree:

```
python3 milestone1/01-ledgerlens/tests/acceptance/run_acceptance.py /path/to/launcher -v
python3 milestone1/01-ledgerlens/tests/acceptance/run_acceptance.py --list     # test ids and the requirements each covers
```

4. The application is accepted only if the suite exits 0 (every test PASS, every requirement PASS) **and** the independent reviewer confirms, from the preserved development history, that the implementation was produced without human-written implementation fixes (D24-E) and that no requirement or test was edited, weakened or removed (REQUIREMENTS.md 1.5). A self-reported pass is insufficient.
5. The suite needs only Python 3 (standard library) and, for the HTTP application, loopback networking. It uses ephemeral ports and temporary directories and removes them afterwards.

## Acceptance criteria by requirement

| Requirement | Acceptance criterion | Acceptance tests |
|---|---|---|
| R1 | Commands, option grammar, version/help text and usage errors (exit 2, empty stdout, no output file) behave exactly as specified; options accepted on either side of INPUT. | `L-R1-version`, `L-R1-help`, `L-R1-usage-errors`, `L-R1-option-order` |
| R2 | BOM, CRLF, blank lines, header case/order/extra columns, quoting, trimming, field-count and UTF-8 handling as specified; malformed structure => exit 65 with the physical line number. | `L-R2-basic-golden`, `L-R2-header-variants`, `L-R2-quoting-and-trim`, `L-R2-utf8`, `L-R2-structure-errors` |
| R3 | Every invalid date/amount/description is rejected with exit 65 naming the line; every boundary-valid value is accepted and canonicalized; sums are exact in cents. | `L-R2-quoting-and-trim`, `L-R3-invalid-values`, `L-R3-valid-values`, `L-R3-exact-cents` |
| R4 | Strict mode aborts on the first bad row (empty stdout, no --out file touched); --lenient skips and counts, header errors stay fatal. | `L-R2-structure-errors`, `L-R3-invalid-values`, `L-R4-strict-no-side-effects`, `L-R4-lenient` |
| R5 | Rules grammar enforced with `rules: line N`; check-rules prints the OK counts line. | `L-R5-check-rules-ok`, `L-R5-check-rules-errors` |
| R6 | First-match priority, unanchored case-sensitive RE2 matching, (?i), Uncategorized fallback, no-rules behavior. | `L-R2-basic-golden`, `L-R6-priority-and-regex`, `L-R6-no-rules`, `L-R12-categorize-golden`, `L-R17-scale-oracle` |
| R7 | Monthly aggregation, TOTAL, money formatting (no -0.00, no float drift). | `L-R2-basic-golden`, `L-R3-exact-cents`, `L-R6-no-rules`, `L-R7-negative-net-format`, `L-R17-scale-oracle` |
| R8 | Category table content and (month, expenses desc, name asc) ordering. | `L-R2-basic-golden`, `L-R6-no-rules`, `L-R8-category-order`, `L-R17-scale-oracle` |
| R9 | Budget rows, strict OVER comparison, refunds do not reduce spend, exit 1 after a complete report. | `L-R2-basic-golden`, `L-R9-budget-semantics`, `L-R14-out-file`, `L-R17-scale-oracle` |
| R10 | --month filter, empty-result report, invalid month => exit 2. | `L-R10-month-filter` |
| R11 | Exact text layout and JSON document shape. | `L-R2-basic-golden`, `L-R11-json-golden` |
| R12 | categorize CSV output, canonical amounts, conditional quoting. | `L-R2-quoting-and-trim`, `L-R3-valid-values`, `L-R6-priority-and-regex`, `L-R12-categorize-golden`, `L-R12-quoting-rules` |
| R13 | INPUT - reads stdin, needs no grants. | `L-R13-stdin` |
| R14 | --out writes identical bytes, empty stdout, same exit code, exit 73 on unwritable path, nothing written on failure. | `L-R4-strict-no-side-effects`, `L-R14-out-file`, `L-R16-write-denied` |
| R15 | --max-rows => exit 70; runner instruction/memory ceilings stop the program with no partial report and no hang. | `L-R15-max-rows`, `L-R15-runtime-limits` |
| R16 | Read/write grant scoping (including `..` and symlink escape), denial => 77 with clean message and no side effects, missing file => 66, works with only the narrow filesystem grants. | `L-R13-stdin`, `L-R16-read-denied`, `L-R16-scope`, `L-R16-write-denied`, `L-R16-minimal-grants` |
| R17 | 5,000-row generated input matches an independent oracle byte-for-byte in values and ordering; repeated runs identical. | `L-R2-basic-golden`, `L-R17-scale-oracle` |
| R18 | stderr prefix on every diagnostic line, silence on success, no raw runtime errors. | `L-R1-usage-errors`, `L-R16-read-denied`, `L-R18-no-stderr-on-success` |

## Test inventory

| Test id | Covers | Description |
|---|---|---|
| `L-R1-version` | R1 | version prints fixed string, needs no grants |
| `L-R1-help` | R1 | help/--help/-h print usage to stdout, exit 0 |
| `L-R1-usage-errors` | R1,R18 | usage errors exit 2, nothing on stdout, no output file |
| `L-R1-option-order` | R1 | options may precede or follow INPUT |
| `L-R2-basic-golden` | R2,R6,R7,R8,R9,R11,R17 | golden summary text for basic fixture, exit 1 (budget over) |
| `L-R2-header-variants` | R2 | header case/order/extra columns/BOM/CRLF/no trailing newline/blank lines |
| `L-R2-quoting-and-trim` | R2,R3,R12 | quoted fields, doubled quotes, trimming, extra columns, signs, 12 digits |
| `L-R2-utf8` | R2 | UTF-8 descriptions pass through byte-exact |
| `L-R2-structure-errors` | R2,R4 | malformed headers/rows abort with exit 65 and physical line numbers |
| `L-R3-invalid-values` | R3,R4 | each invalid date/amount aborts with exit 65 naming the line |
| `L-R3-valid-values` | R3,R12 | boundary-valid dates/amounts accepted and canonicalized |
| `L-R3-exact-cents` | R3,R7 | sums are exact (no float drift), large values exact |
| `L-R4-strict-no-side-effects` | R4,R14 | strict failure: empty stdout, no --out file, diagnostic names line |
| `L-R4-lenient` | R4 | --lenient skips invalid rows, counts them, header errors stay fatal |
| `L-R5-check-rules-ok` | R5 | check-rules OK line |
| `L-R5-check-rules-errors` | R5 | rules errors name the physical line, exit 65 |
| `L-R6-priority-and-regex` | R6,R12 | first matching rule wins, unanchored, case-sensitive, (?i) |
| `L-R6-no-rules` | R6,R7,R8 | summary without --rules: everything Uncategorized, no budgets section |
| `L-R8-category-order` | R8 | category rows: month, expenses desc, name asc |
| `L-R9-budget-semantics` | R9 | equal is OK, refunds do not reduce spend, Uncategorized budget, zero-spend rows |
| `L-R10-month-filter` | R10 | --month filters; no match gives empty report; bad month is usage error |
| `L-R11-json-golden` | R11 | JSON document shape and values for basic fixture |
| `L-R7-negative-net-format` | R7 | negative net formatting, income-only month, no '-0.00' |
| `L-R12-categorize-golden` | R12,R6 | categorize golden output |
| `L-R12-quoting-rules` | R12 | category/description quoting only when needed |
| `L-R13-stdin` | R13,R16 | INPUT '-' reads stdin; no grants needed without --rules |
| `L-R14-out-file` | R14,R9 | --out writes the stdout bytes, keeps exit code, stdout empty |
| `L-R15-max-rows` | R15 | --max-rows limit exit 70, empty stdout, no file |
| `L-R15-runtime-limits` | R15 | tiny instruction/memory ceilings stop the run promptly with no partial report |
| `L-R16-read-denied` | R16,R18 | no grants: reading input/rules file is a clean denial (77) |
| `L-R16-scope` | R16 | read grant is scoped to its root; paths outside are denied; missing inside is 66 |
| `L-R16-write-denied` | R16,R14 | read-only grant: --out is denied (77), nothing created; write-only grant cannot read |
| `L-R16-minimal-grants` | R16 | works with ONLY the narrow filesystem grants (no network/process/env/db) |
| `L-R17-scale-oracle` | R17,R7,R8,R9,R6 | 5,000 generated rows match an independent Python oracle; output deterministic |
| `L-R18-no-stderr-on-success` | R18 | successful runs write nothing to stderr |

## Capability and resource evidence (D24-H)

The tests tagged with the capability requirement(s) in the table above exercise the actual runtime grants and denials (including denial with an empty grant list, scoped grants, and the absence of side effects after a denial) and the runner resource limits through `HF_RUN_FLAGS`. Preserve the suite output together with a `--receipt` of at least one denied run per application as additional evidence; the suite itself does not require receipts.
