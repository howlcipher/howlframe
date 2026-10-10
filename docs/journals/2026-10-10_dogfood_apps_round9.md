# Dogfood common apps round 9 (2026-10-10)

## Goal

Empty-string and unix-time density sugar for general CLI/API use; ship now/sleep apps.

## Blockers found and fixed

| Gap | Symptom vs other languages | Fix |
|---|---|---|
| Repeated `(= x "")` | Verbose empty checks vs falsy/`len==0` idioms. | `(blank x)` sugar. |
| Verbose unix timestamp | `(to_string (time_now))` repeats in clock APIs. | `(unix_now)` sugar. |

## Apps shipped / densified

1. **`apps/now_cli`** — print unix time (15 tokens)
2. **`apps/sleep_cli`** — sleep ms then `OK`
3. Densified `stamp_api` (`unix_now`), `env_cli`/`mkdir_cli`/`join_cli`/`uniq_cli` (`blank`)

## Token proof (`cl100k_base`, 26 apps)

| | HowlFrame | Python | Go | Node |
|---|---|---|---|---|
| **TOTAL** | **3814** | 5172 | 5491 | 4983 |

```bash
python3 benchmarks/real_apps/prove_real_apps.py
```

## Remaining limits

- `middleware` still Unsupported
- `match` still Unsupported (prod-flip lock)
- `schema` / SQL migrations still Unsupported
- No native `to_lower`/`to_upper`/`trim` yet
