# Dogfood common apps round 10 (2026-10-10)

## Goal

Native case/trim CONVERT ops for general string use; ship lower/upper/trim/icontains apps.

## Blockers found and fixed

| Gap | Symptom vs other languages | Fix |
|---|---|---|
| No `to_lower`/`to_upper`/`trim` | Case folds need host escapes or awkward splits. | Native CONVERT: `to_lower`, `to_upper`, `str_trim`. |
| Case-insensitive search verbose | Manual lower on both sides repeats. | `icontains_cli` dogfoods `to_lower` + `str_contains`. |

## Apps shipped / densified

1. **`apps/lower_cli`** — lowercase argv text
2. **`apps/upper_cli`** — uppercase argv text
3. **`apps/trim_cli`** — trim argv text
4. **`apps/icontains_cli`** — case-insensitive file substring
5. Densified `starts_cli`/`ends_cli`/`contains_cli`/`template_cli`/`file_transform` with `blank`

## Token proof

```bash
python3 benchmarks/real_apps/prove_real_apps.py
```

## Remaining limits

- `middleware` still Unsupported
- `match` still Unsupported (prod-flip lock)
- `schema` / SQL migrations still Unsupported
