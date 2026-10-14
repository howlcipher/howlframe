# REQUIREMENTS: ledgerlens (Milestone 1, application 1 of 3)

> **STATUS: FROZEN, dated 2026-10-13.** Changes require a separately approved requirements revision (see 1.5). Tool name: `ledgerlens`. Domain: personal/small-business finance (bank-export analysis). Unrelated to the Howl ecosystem.

## 0. Problem statement

People receive bank/card exports as CSV with messy descriptions and want a private, offline answer to: "where did my money go each month, and did I stay within budget?" `ledgerlens` reads a transaction CSV (file or stdin) and a plain-text rules file, assigns each transaction to a category using ordered regular-expression rules, and produces monthly and per-category summaries and budget checks as text or JSON, or a normalized categorized CSV. Money is handled exactly (integer cents, never binary floating point). It must read only what it is granted, write only where it is granted, and fail cleanly on bad data.

## 1. Common conventions (apply to every Milestone 1 application)

### 1.1 Deliverable layout and build

The implementation lives under `milestone1/01-ledgerlens/src/` (`.howl` source; HFIR is permitted if it qualifies under directive D24-B). The program MUST be built to a standalone HowlFrame artifact:

```
howlframe build milestone1/01-ledgerlens/src/ledgerlens.howl -o milestone1/01-ledgerlens/build/ledgerlens.hfbc
```

The artifact MUST execute on the standalone VM only (`howlframe run`, or legacy `-run-bc` with an equivalent `-compile-bc` artifact). It MUST NOT depend on generated Go/Python/JavaScript, on helper scripts, or on any external program at run time unless this document says so.

### 1.2 Invocation convention ("the launcher")

All behavior is specified for the invocation `ledgerlens <args...>`, which the acceptance suite performs through a **launcher**: an executable file with this contract:

```
LAUNCHER GRANTS [APP_ARG...]
```

* `GRANTS` is exactly ONE argument: the comma-separated runtime grant list for `--allow-caps` (for example `filesystem:read=/tmp/x,filesystem:write=/tmp/x`). The empty string means "no grants": the launcher MUST then pass no `--allow-caps` at all.
* `APP_ARG...` are passed to the application verbatim, as its `cli_args`, in order, with no re-quoting and no interpretation of flag-looking arguments.
* The environment variable `HF_RUN_FLAGS` (whitespace-separated runner flags such as `--max-instructions 5000`, `--max-fetch-bytes 1024`, `--deadline 1s`, `--max-exec-output-bytes 100`) MUST be inserted by the launcher as runner options before the artifact path. It is the only channel for resource limits.
* The launcher forwards its stdin, stdout, stderr, working directory and process environment unchanged and exits with the application's exit status.

Reference launcher (`tests/acceptance/launcher.example.sh`):

```
howlframe run [--allow-caps "$GRANTS"] $HF_RUN_FLAGS milestone1/01-ledgerlens/build/ledgerlens.hfbc "$@"
```

Unless a test says otherwise the acceptance suite sets `HF_RUN_FLAGS="--max-instructions 4000000000 --max-memory-bytes 4294967296 --max-call-depth 10000"` (generous but finite ceilings; the runtime defaults of 100000 instructions / 64 MiB / call depth 1000 are far too small for realistic input and are NOT an application requirement). Resource-limit tests substitute smaller values.

Grant syntax (see `docs/cli.md`, `docs/reference/network_authority.md`, `docs/reference/process_authority.md`): `filesystem:read=<abs dir>`, `filesystem:write=<abs dir>`, `network:fetch=<host>:<port>`, `process:exec=<abs program> <pattern...>`, `process:env=<NAME>`, `environment`. The application never receives more than the grants it is launched with and MUST NOT attempt to widen them.

### 1.3 Streams and exit codes

* Results go to **stdout**. Diagnostics go to **stderr**; every diagnostic line MUST start with `ledgerlens: `. Because the runtime's `stderr` built-in adds no newline, the application MUST terminate each diagnostic line with `\n`.
* stdout and stderr are UTF-8; text is passed through byte-exact (no re-encoding, no case-folding of non-ASCII).
* Exit codes (all applications):

| Code | Meaning |
|---|---|
| 0 | Success, nothing to report as a finding |
| 1 | Success of the tool, but the analysis found failures/violations (defined per application) |
| 2 | Usage error (bad command, bad/missing/duplicate option, wrong operand count). Nothing else is done: no input read, no file written |
| 65 | Input data/format/config error (data is unusable) |
| 66 | A required input file does not exist or is not a regular readable file |
| 70 | A resource limit was reached (application-level limit or a runtime `LIMIT_EXCEEDED` the application is able to intercept) |
| 73 | An output file cannot be written (and the failure is not a capability denial) |
| 77 | Capability denied by the runtime (see 1.4) |

### 1.4 Capability denial behavior (uniform)

When the runtime refuses an operation with `CAPABILITY_DENIED` the application MUST:

1. stop the operation that needed it, with no partial side effects (no partially written output file; nothing destructive done);
2. print exactly one diagnostic line of the form `ledgerlens: capability denied: <capability-name>[ (<what>)]` on stderr, where `<capability-name>` is one of `filesystem`, `network`, `process`, `environment`, `database`;
3. exit with status 77 (unless this document defines a different, explicitly stated, per-item handling);
4. NEVER print raw runtime error objects (strings containing `"phase":`), host stack traces, or the text of secret values.

Missing files are NOT denials (exit 66, or as specified). Operations that do not need a capability MUST work with an empty grant list.

### 1.5 Freeze notice

**FROZEN 2026-10-13.** This document, together with `ACCEPTANCE.md` and `tests/acceptance/`, was written by the independent selector/evaluator before any implementation. Changes require a separately approved requirements revision that is recorded as such (new revision header, dated, with rationale); implementers MUST NOT edit, dilute or remove requirements or tests to claim success. The freeze is sha-agnostic: it binds to the content as authored, not to a repository revision.

## 2. Command-line interface

```
ledgerlens summary    [--rules FILE] [--month YYYY-MM] [--format text|json] [--out FILE] [--lenient] [--max-rows N] INPUT
ledgerlens categorize --rules FILE [--out FILE] [--lenient] [--max-rows N] INPUT
ledgerlens check-rules FILE
ledgerlens version
```

* `INPUT` is a path, or `-` meaning standard input. Exactly one `INPUT` per `summary`/`categorize`.
* Options take their value as the NEXT argument (`--rules r.ini`); the `--opt=value` form is NOT supported and is a usage error. Options may appear before or after `INPUT`. An option given twice, an unknown option, a missing option value, a command-inappropriate option (e.g. `--month` on `categorize`, `--format` on `categorize`), or a missing/extra operand is a usage error (exit 2). `--format` defaults to `text`; any other value than `text`/`json` is exit 2. No command, an unknown command, or `help`/`--help`/`-h` as the first argument prints a usage text to **stdout** for `help`/`--help`/`-h` with exit 0 (the first line of that text MUST begin with `usage: ledgerlens`), and is exit 2 (message on stderr) for no command or an unknown command.
* `version` takes no operands; `check-rules` takes exactly one operand (the rules file); `help` takes none.
* `version` prints exactly `ledgerlens 1.0.0` and a newline to stdout, exit 0, and needs no grants.
* `--max-rows N`: positive decimal integer (`1`..`10000000`), default `100000`. Anything else is exit 2.
* Exit codes are those of section 1.3. For this tool exit 1 means "at least one budget is exceeded" (R9). There are no other conditions that produce exit 1.

## 3. Input formats

### 3.1 Transaction CSV (`INPUT`)

* Optional UTF-8 BOM at the very start is ignored. Lines end with `\n` or `\r\n`; a trailing `\r` is removed. Lines that are empty after that (zero length) are skipped, but physical line numbers (1-based) still count them. A line of only spaces is NOT empty: it is a malformed data row (one field, wrong field count).
* The first non-skipped line is the header: comma-separated names, each trimmed of surrounding spaces and compared case-insensitively (ASCII). It MUST contain each of `date`, `description`, `amount` exactly once; column order is free; any other columns are allowed and ignored (their names may repeat). Missing, duplicated, or unquoted-field-malformed header => exit 65 (`line 1: ...`). An input with no header line at all (empty, or only blank lines) => exit 65 `empty input`.
* Data rows have exactly as many fields as the header. Field syntax (RFC 4180 subset): fields are separated by `,`; a field starting with `"` is quoted: it ends at the next `"` that is not followed by another `"`, `""` inside means one literal `"`, commas inside are literal, and after the closing quote the next character MUST be `,` or end of line. A `"` anywhere in an unquoted field is an error. A quoted field cannot contain a line break (a quote left open at end of line is an error `unterminated quote`). Surrounding spaces of UNQUOTED fields are trimmed; quoted content is verbatim.
* `date`: `YYYY-MM-DD`, a real calendar date (leap years honored: `2024-02-29` valid, `2023-02-29` invalid), year `1900`..`2999`.
* `amount`: after trimming, matches `[+-]?[0-9]+(\.[0-9]{1,2})?` with at most 12 integer digits. No currency symbols, no thousands separators, no exponent. Negative = money out (expense), positive = money in (income), zero allowed. Stored exactly in integer cents (`12.5` = 1250 cents).
* `description`: any text (possibly empty), UTF-8, up to 1000 bytes (longer is an invalid row).

### 3.2 Rules file (`--rules FILE`, `check-rules FILE`)

Plain text, UTF-8, `\n` or `\r\n` line endings. Blank lines and lines whose first non-space character is `#` are ignored. Section headers are exactly `[categories]` or `[budgets]` (surrounding spaces ignored, case-sensitive). Every other non-ignored line MUST be `NAME = VALUE`, split at the FIRST `=`, both sides trimmed, appearing after a section header.

* In `[categories]`: `NAME` matches `[A-Za-z][A-Za-z0-9 _-]{0,39}` and is not `Uncategorized`; `VALUE` is a non-empty regular expression in Go/RE2 syntax, matched UNANCHORED (a search) against the transaction description, case-sensitive unless the pattern says `(?i)`. The same `NAME` may appear on several lines (several patterns for one category). **Rule order in the file is priority order across all category lines.**
* In `[budgets]`: `NAME` is a category that has at least one line in `[categories]`, or `Uncategorized`; `VALUE` matches `[0-9]+(\.[0-9]{1,2})?` and is greater than zero. A `NAME` may appear only once in `[budgets]`. A budget is a per-calendar-month limit on that category's expenses.
* Either section may be absent or empty. Both sections may appear multiple times (their entries concatenate in order).
* Any violation (entry outside a section, bad name, empty or non-compiling regex, bad budget value, unknown or duplicate budget name, malformed line) => exit 65 with `ledgerlens: rules: line N: <message>` on stderr, N being the 1-based physical line. `check-rules` validates the whole file; the first error found (lowest line number) is reported.

## 4. Functional requirements

**R1 (CLI shell).** Command dispatch, option parsing, `version`, `help`, usage errors and exit codes exactly as in section 2. Usage errors write nothing to stdout, read no input and create no file.

**R2 (CSV parsing).** Section 3.1 parsing rules: BOM, CRLF, skipped blank lines, header detection and any column order, extra columns ignored, quoting rules, trimming rules, field-count check. Descriptions are carried byte-exact (including UTF-8 and embedded commas/quotes).

**R3 (field validation).** `date` and `amount` and `description` validation as in 3.1; exact cent arithmetic; `+`/`-` signs; `-0.00` is zero.

**R4 (error policy).** In the default (strict) mode the FIRST invalid data row aborts the run: exit 65, stderr `ledgerlens: line N: <message>` (N = physical line of the offending row), **empty stdout, and no `--out` file created or modified**. With `--lenient`, invalid data rows (field count, quoting, date, amount, description length) are skipped and counted, processing continues, and a final stderr line `ledgerlens: skipped K invalid row(s)` is written only when K > 0; header errors and empty input remain fatal (exit 65). Rows skipped in lenient mode do not count as transactions.

**R5 (rules).** Rules parse and validation as in 3.2. `check-rules FILE` prints `OK: C rule(s), B budget(s)` (counts of category lines and budget lines) and exits 0, or reports as in 3.2 (exit 65).

**R6 (categorization).** A transaction's category is the name on the FIRST category line (file order) whose regex matches its description; otherwise `Uncategorized`. Without `--rules`, every transaction is `Uncategorized` and there are no budgets. Regex semantics are Go/RE2, unanchored.

**R7 (monthly summary).** The month of a transaction is the first 7 characters of its date. For each month with at least one transaction: `income` = sum of positive amounts, `expenses` = sum of absolute values of negative amounts, `net` = income - expenses, `count` = number of transactions (including zero-amount). A `TOTAL` aggregate over the listed months is also produced. Money is formatted `[-]D.CC`: integer part without leading zeros or grouping, always exactly two decimals, `-` only for negative values (never `-0.00`). Months are listed in ascending lexicographic order.

**R8 (category table).** For every (month, category) with at least one transaction: `income`, `expenses`, `count` as in R7 restricted to that category. Rows sorted by month ascending, then expenses descending (numeric), then category name ascending (byte order). The category `Uncategorized` sorts like any other name.

**R9 (budgets).** Only when the rules define at least one budget: for every listed month and every budget category (sorted by category name, byte order) one row with `limit`, `spent` (= that category's expenses in that month, `0.00` if none), and `status` = `OVER` if `spent` > `limit` (strictly), else `OK`. If any row is `OVER` the program still prints the full report and then exits **1**; otherwise 0. Refunds (positive amounts) never reduce `spent`.

**R10 (month filter).** `--month YYYY-MM` (valid month `01`..`12`, year 1900..2999, else exit 2) restricts the report to transactions in that month. If none match, the report is produced with no month rows, a TOTAL of zeros, `transactions` 0, and (if budgets are defined) no budget rows.

**R11 (output formats).** `summary` text format (exact):

```
# monthly
month<TAB>income<TAB>expenses<TAB>net<TAB>transactions
2026-01<TAB>3000.00<TAB>1234.50<TAB>1765.50<TAB>12
TOTAL<TAB>3000.00<TAB>1234.50<TAB>1765.50<TAB>12
<empty line>
# categories
month<TAB>category<TAB>income<TAB>expenses<TAB>transactions
2026-01<TAB>Rent<TAB>0.00<TAB>1000.00<TAB>1
<empty line>
# budgets
month<TAB>category<TAB>limit<TAB>spent<TAB>status
2026-01<TAB>Rent<TAB>900.00<TAB>1000.00<TAB>OVER
```

Sections are separated by exactly one empty line; the `# budgets` section (and its preceding empty line) is omitted when no budgets are defined; the output ends with a single `\n`. `--format json` prints one JSON document:

```
{"transactions": N, "skipped": K,
 "months":     [{"month":"2026-01","income":"3000.00","expenses":"1234.50","net":"1765.50","count":12}, ...],
 "total":      {"income":"...","expenses":"...","net":"...","count":12},
 "categories": [{"month":"2026-01","category":"Rent","income":"0.00","expenses":"1000.00","count":1}, ...],
 "budgets":    [{"month":"2026-01","category":"Rent","limit":"900.00","spent":"1000.00","status":"OVER"}, ...]}
```

Money values are strings, counts are JSON integers, `budgets` is `[]` when no budgets exist, row orders are as in R7-R9. Whitespace in the JSON is free; the document is followed by a newline.

**R12 (categorize).** `categorize` prints a normalized CSV to stdout: header line `date,description,amount,category`, then one line per transaction in input order. `amount` is canonical `[-]D.CC` (a leading `+` is dropped, `-0.00` becomes `0.00`). `description` and `category` are written in double quotes (with `"` doubled) if and only if they contain `,` or `"` or have leading/trailing spaces; otherwise bare. Lines end with `\n`. Extra input columns are dropped. Exit 0 always on success (there are no budget semantics here).

**R13 (stdin).** `INPUT` = `-` reads the whole standard input; semantics identical to a file with the same bytes. Reading stdin needs no capability. A rules file given with `--rules` still needs filesystem read access.

**R14 (output file).** `--out FILE` writes exactly the bytes that would have gone to stdout into FILE (create or truncate), leaves stdout empty, and keeps the same exit code as without `--out` (including 1 for budgets). The file is written only once, after the whole report was computed successfully; on any error nothing is created or altered. If FILE cannot be written for reasons other than a capability denial (e.g. its directory does not exist) => exit 73 and stderr `ledgerlens: cannot write <FILE>`.

**R15 (limits).** (a) Application limit: if the number of data rows (non-empty lines after the header, valid or not) exceeds `--max-rows`, exit 70 with `ledgerlens: row limit exceeded (max N)`, empty stdout, no output file. (b) Runtime limits: when the runner stops the program for `--max-instructions` / `--max-memory-bytes` / `--deadline`, the program terminates promptly with a non-zero status and MUST NOT have printed any part of a report (the report is emitted only after all computation finished; stdout stays empty). It must never hang or loop forever on large input.

**R16 (capabilities).** Required grants and expected denials:

| Operation | Required grant | Denied behavior |
|---|---|---|
| read `INPUT` file | `filesystem:read=<dir containing it>` (or `filesystem`) | exit 77, section 1.4 |
| read rules file | `filesystem:read=<dir>` | exit 77 |
| write `--out` | `filesystem:write=<dir>` (read access is NOT implied and NOT needed) | exit 77, file not created |
| `version`, `help`, usage errors, `INPUT` = `-` without `--rules` | none | works with empty grants |

A path outside every granted root is a denial (exit 77), not a "file missing". Denial MUST be reported even when the file does not exist (the program must not probe the path first by other means). When an input is denied, no stdout. The program MUST NOT require `network`, `process`, `environment` or `database`.

**R17 (determinism, scale).** Same inputs => byte-identical output. A 5,000-row input with ~12 categories and budgets must be processed within the suite's time cap (120 s) under the suite's generous runner ceilings.

**R18 (diagnostic hygiene).** Every stderr line is prefixed `ledgerlens: `; a failure produces no raw runtime error objects, stack traces or secrets; successful runs without `--lenient` skips write nothing to stderr.

## 5. Edge cases (all binding; each is covered by R-ids above)

Leap-day dates; `2026-13-01`, `2026-00-10`, `2026-04-31`, `26-01-01`, `2026-1-5` invalid; amounts `.5`, `5.`, `1e3`, `1,000.00`, `$5`, `1.234`, ` 5.00 ` (trim OK) ; `+0`, `-0`, `0.00`; 12-digit amounts exact; header with different case/order/extra columns/BOM; CRLF input; no trailing newline at EOF; blank lines (line numbers still physical); description with commas, doubled quotes, UTF-8, empty; quoted field followed by junk; field-count mismatch; duplicate required header; rules with `=` inside the regex; `(?i)` patterns; invalid regex; duplicate budget; budget for unknown category; budget equal to spent (OK, not OVER); `Uncategorized` budget; months with only income; `--month` with no matches.

## 6. Out of scope

Multiple currencies, currency symbols, locale number formats, thousands separators, date formats other than ISO, embedded line breaks in quoted fields, reading multiple inputs, editing or writing back rules, an interactive mode, network access, any database, colored output, `--opt=value` syntax, partial-month proration, recurring-transaction detection.
