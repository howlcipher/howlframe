# Dogfood common apps round 8 (2026-10-10)

## Goal

HTTP response sugar, boolean `not`, join/uniq/echo apps for general CLI/API use.

## Blockers found and fixed

| Gap | Symptom vs other languages | Fix |
|---|---|---|
| Verbose HTTP status | `(map_get r "status")` repeats with body decode. | `(http_status r)` sugar. |
| Verbose proxy res | Full `res`+status+ctype+body boilerplate. | `(http_res r)` → JSON `res` from `http_request` result. |
| No boolean not | Cannot invert `list_contains` cleanly. | `(not x)` sugar. |

## Apps shipped / densified

1. **`apps/join_cli`** — join lines with separator
2. **`apps/uniq_cli`** — unique lines (`list_contains` + `not`)
3. **`apps/echo_api`** — minimal JSON echo API
4. Densified `fetch_cli` (`http_status`) and `webhook_relay` (`http_res`)

## Token proof (`cl100k_base`, 24 apps)

| | HowlFrame | Python | Go | Node |
|---|---|---|---|---|
| **TOTAL** | **3743** | 5037 | 5341 | 4862 |

```bash
python3 benchmarks/real_apps/prove_real_apps.py
```

## Remaining limits

- `middleware` still Unsupported
- `match` still Unsupported (prod-flip lock)
- `schema` / SQL migrations still Unsupported
- `fetch` still body-bytes only
