# Dogfood common apps round 5 (2026-10-10)

## Goal

Density sugar for substring work, densify existing apps, ship mkdir/contains/file_transform into the prove suite.

## Blockers found and fixed

| Gap | Symptom vs other languages | Fix |
|---|---|---|
| No substring test | `str_split`+`list_len` tax vs `in` / `includes` / `Contains`. | `(str_contains hay needle)` sugar. |
| No replace | Manual split/join vs `replace` / `strings.Replace`. | `(str_replace s old new)` → `str_join` of split. |

## Apps shipped / densified

1. **`apps/mkdir_cli`** — recursive mkdir
2. **`apps/contains_cli`** — substring search (`str_contains`)
3. **`apps/file_transform`** — densified with `when`/`inc`/`str`; added to prove suite
4. **`apps/template_cli`** — densified with `str_contains`/`str_replace` (184 → 160 tokens)

## Token proof (`cl100k_base`, 15 apps)

| | HowlFrame | Python | Go | Node |
|---|---|---|---|---|
| **TOTAL** | **2716** | 3539 | 3850 | 3413 |

```bash
python3 benchmarks/real_apps/prove_real_apps.py
```

## Remaining limits

- `middleware` still Unsupported
- `match` still Unsupported (`switch`/`cond` cover dispatch; prod-flip lock)
- `schema` / SQL migrations still Unsupported
- `fetch` still body-bytes only (`http_request` for status+body)
