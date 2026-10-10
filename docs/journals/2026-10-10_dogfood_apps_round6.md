# Dogfood common apps round 6 (2026-10-10)

## Goal

Prefix/suffix density sugar for general use, densify text CLIs with `read_text`, ship env/starts/ends apps.

## Blockers found and fixed

| Gap | Symptom vs other languages | Fix |
|---|---|---|
| No prefix test | Manual `str_split`/`list_get` vs `startswith` / `HasPrefix` / `startsWith`. | `(starts_with s p)` sugar. |
| No suffix test | Same vs `endswith` / `HasSuffix` / `endsWith`. | `(ends_with s p)` sugar (sentinel + `str_contains`). |
| Verbose `read_file`+`bytes_to_string` | Already had `read_text` but apps underused it. | Densify contains/template/json_get. |

## Apps shipped / densified

1. **`apps/env_cli`** — print env var or `MISSING`
2. **`apps/starts_cli`** — file prefix check
3. **`apps/ends_cli`** — file suffix check
4. Densified `contains_cli`, `template_cli`, `json_get` via `read_text`

## Token proof (`cl100k_base`, 18 apps)

| | HowlFrame | Python | Go | Node |
|---|---|---|---|---|
| **TOTAL** | **3002** | 3958 | 4248 | 3827 |

```bash
python3 benchmarks/real_apps/prove_real_apps.py
```

## Remaining limits

- `middleware` still Unsupported
- `match` still Unsupported (prod-flip lock)
- `schema` / SQL migrations still Unsupported
- `fetch` still body-bytes only
