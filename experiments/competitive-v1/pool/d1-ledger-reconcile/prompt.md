# Dual-Ledger Financial Reconciliation

Build a reconciliation CLI tool that compares an internal accounting transaction journal against an external settlement bank CSV export, matches transactions, computes net settled funds, and isolates discrepancies.

## CLI Interface

The executable is located at `./run`. It must implement the `reconcile` subcommand:

```bash
./run reconcile --internal <internal_json> --external <external_csv> --out <recon_json>
```

Arguments:
- `reconcile`: Required subcommand. If absent or different, exit with code `1`.
- `--internal <internal_json>`: Path to internal transaction records in JSON format. Required.
- `--external <external_csv>`: Path to external settlement statement in CSV format. Required.
- `--out <recon_json>`: Path where the reconciliation output report will be written. Required.

Options may be passed in any order.

### Exit Codes
- `0`: Success, reconciliation completed and report written.
- `1`: Invalid CLI arguments, missing required options, or unrecognized subcommand.
- `2`: Either the `--internal` or `--external` file does not exist or cannot be read.

## Input File Formats

### 1. Internal Journal (`--internal`)
A JSON array of objects representing recorded transactions:
```json
[
  {
    "txn_id": "TX-1001",
    "account_id": "ACC-CORP-1",
    "amount": 250.75,
    "currency": "USD",
    "timestamp": "2026-10-01T14:30:00Z"
  }
]
```

### 2. External Statement (`--external`)
A CSV file with header:
`ref_id,account_id,amount,currency,posted_at`

Example:
```csv
ref_id,account_id,amount,currency,posted_at
TX-1001,ACC-CORP-1,250.75,USD,2026-10-01T15:00:00Z
TX-1002,ACC-CORP-2,80.00,EUR,2026-10-01T15:05:00Z
```

## Reconciliation Logic

1. **Duplicate Detection**:
   - Parse all records. If a `txn_id` appears more than once within the internal dataset, record an entry in `duplicates`: `{"source": "internal", "id": "<txn_id>"}`.
   - If a `ref_id` appears more than once within the external dataset, record an entry in `duplicates`: `{"source": "external", "id": "<ref_id>"}`.
   - Duplicate entries should be excluded from subsequent matching and discrepancy comparisons.

2. **Matching & Discrepancies (for non-duplicate records)**:
   - Attempt to match internal `txn_id` with external `ref_id`:
     - **Matched**: Both records exist, and `account_id`, `currency`, and `amount` (compared rounded to 2 decimal places) match exactly. Increment `matched_count` and add `amount` to `net_settled_amount`.
     - **Metadata Mismatch**: Both records exist with the same ID, but either `account_id` or `currency` does not match:
       ```json
       {
         "id": "<id>",
         "internal_account": "<internal_account>",
         "external_account": "<external_account>",
         "internal_currency": "<internal_currency>",
         "external_currency": "<external_currency>"
       }
       ```
     - **Amount Mismatch**: Both records exist with identical `account_id` and `currency`, but the amounts differ:
       ```json
       {
         "id": "<id>",
         "account_id": "<account_id>",
         "currency": "<currency>",
         "internal_amount": <internal_amount>,
         "external_amount": <external_amount>,
         "difference": <round(internal_amount - external_amount, 2)>
       }
       ```
     - **Unmatched Internal**: Record exists in internal journal but has no matching `ref_id` in external statement:
       ```json
       {
         "id": "<id>",
         "account_id": "<account_id>",
         "amount": <amount>,
         "currency": "<currency>"
       }
       ```
     - **Unmatched External**: Record exists in external statement but has no matching `txn_id` in internal journal:
       ```json
       {
         "id": "<id>",
         "account_id": "<account_id>",
         "amount": <amount>,
         "currency": "<currency>"
       }
       ```

## Output Format (`--out`)

Write a JSON report to `<recon_json>`:

```json
{
  "matched_count": 1,
  "net_settled_amount": 250.75,
  "discrepancies": {
    "amount_mismatch": [],
    "metadata_mismatch": [],
    "unmatched_internal": [],
    "unmatched_external": [],
    "duplicates": []
  }
}
```

Formatting rules:
- `net_settled_amount` must be rounded to 2 decimal places.
- All lists inside `discrepancies` must be sorted ascending by `id` (code point order).
- `duplicates` must be sorted by `source` ascending (`"external"` before `"internal"`), then `id` ascending.
