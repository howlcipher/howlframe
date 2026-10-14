# ledgerlens

Offline bank-export analysis written in HowlFrame. Reads a transaction CSV (file or
stdin) and an optional rules file, assigns categories with ordered regular expressions,
and prints monthly and per-category summaries and budget checks (text or JSON), or a
normalized categorized CSV. Money is exact integer cents.

Specification (frozen 2026-10-13): `requirements/REQUIREMENTS.md`.

## Build and run

```
go build -o howlframe .                                   # from the repository root
./howlframe build milestone1/01-ledgerlens/src/ledgerlens.howl -o ledgerlens.hfbc
./howlframe run --allow-caps filesystem:read=$PWD ledgerlens.hfbc summary --rules rules.ini txns.csv
```

Runner flags (`--allow-caps`, `--max-instructions`, ...) go before the artifact. The runtime
defaults (100000 instructions, 64 MiB) are too small for real exports; use for example
`--max-instructions 4000000000 --max-memory-bytes 4294967296 --max-call-depth 10000`.

## Commands

```
ledgerlens summary    [--rules FILE] [--month YYYY-MM] [--format text|json] [--out FILE] [--lenient] [--max-rows N] INPUT
ledgerlens categorize --rules FILE [--out FILE] [--lenient] [--max-rows N] INPUT
ledgerlens check-rules FILE
ledgerlens version
```

`INPUT` is a path or `-` for stdin. Exit codes: 0 ok, 1 a budget is exceeded, 2 usage,
65 bad data or rules, 66 missing file, 70 limit, 73 write failed, 77 capability denied.

## Grants

| Operation | Grant |
|---|---|
| read INPUT or rules | `filesystem:read=<dir>` |
| write `--out` | `filesystem:write=<dir>` (read is not implied) |
| `version`, usage errors, `INPUT` = `-` without `--rules` | none |

It never needs network, process, environment, or database access.

## Rules file

```
[categories]          # order is priority; first match wins; unanchored RE2, case-sensitive unless (?i)
Rent = ^ACME PROPERTIES
Groceries = (?i)whole foods|trader joe
[budgets]             # per-month limit on expenses of that category
Groceries = 300.00
```

## Tests

* Frozen black-box suite: `python3 tests/acceptance/run_acceptance.py tests/acceptance/launcher.example.sh`
  (set `HF_BIN` and `APP_ARTIFACT`).
* Clean-environment reproduction from the committed tree: `scripts/clean_repro.sh`.

## Known limitations

* Character-based length: the 1000-byte description cap is exact for descriptions over 250
  characters and trivially satisfied below that. (`str_len` counts runes.)
* The source contains no non-ASCII string literals, because the lexer double-encodes them.
  Data read at run time is byte-exact.
* Output is built in memory and written once, so very large reports are bounded by the
  runner's memory ceiling.
* Budget amounts are capped at 12 integer digits.
