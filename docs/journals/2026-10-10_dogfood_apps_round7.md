# Dogfood common apps round 7 (2026-10-10)

## Goal

HTTP body and list membership sugar, decrement symmetry, more Unix-style CLIs, densify fetch/webhook.

## Blockers found and fixed

| Gap | Symptom vs other languages | Fix |
|---|---|---|
| Verbose HTTP body decode | `(bytes_to_string (map_get r "body"))` repeats. | `(http_body r)` sugar. |
| No list membership | Manual for-scan vs `in` / `slices.Contains` / `includes`. | `(list_contains lst x)` sugar. |
| No decrement | `inc` without `dec`. | `(dec x)` sugar. |

## Apps shipped / densified

1. **`apps/head_cli`** — first N lines
2. **`apps/cut_cli`** — CSV column extract
3. **`apps/has_line_cli`** — exact line membership (`list_contains`)
4. Densified `fetch_cli` and `webhook_relay` with `http_body` (fetch 109 → 102)

## Token proof (`cl100k_base`, 21 apps)

| | HowlFrame | Python | Go | Node |
|---|---|---|---|---|
| **TOTAL** | **3413** | 4461 | 4813 | 4345 |

```bash
python3 benchmarks/real_apps/prove_real_apps.py
```

## Remaining limits

- `middleware` still Unsupported
- `match` still Unsupported (prod-flip lock)
- `schema` / SQL migrations still Unsupported
- `fetch` still body-bytes only (`http_request` for status+body)
