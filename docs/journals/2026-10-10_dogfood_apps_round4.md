# Dogfood common apps round 4 (2026-10-10)

## Goal

Continue dogfooding: density sugar for general use, more common apps, prove tokens.

## Blockers found and fixed

| Gap | Symptom vs other languages | Fix |
|---|---|---|
| No dict membership | `map_get` miss is `""`; cannot tell missing vs empty. | `(map_has d k)` sugar via `map_keys`. |
| Binary-only `and`/`or` | `(or a b c)` rejected; Python/JS allow n-ary. | Desugar n-ary `and`/`or` to nested binary. |
| Verbose single-branch if | `if` always wants else for clarity in CLIs. | `(when test body…)` / `(unless test body…)`. |

## Apps shipped

1. **`apps/fetch_cli`** — outbound HTTP status+body
2. **`apps/template_cli`** — `{{key}}` replacement
3. **`apps/stamp_api`** — `time_now` clock API

Also densified `json_get` with `map_has`. Prove harness always rebuilds `howlframe`.

## Token proof

| TOTAL (12 apps) | HowlFrame | Python | Go | Node |
|---|---|---|---|---|
| **TOTAL** | **2331** | 3025 | 3295 | 2927 |

```bash
python3 benchmarks/real_apps/prove_real_apps.py
```

## Follow-on

- Round 5 continues in `docs/journals/2026-10-10_dogfood_apps_round5.md`
