# Dogfood common apps round 11 (2026-10-10)

## Goal

General-use density sugar for dict defaults, numeric min/max, case-insensitive
equality, and store next-id allocation; ship dogfood CLIs and densify store/HTTP apps.

## Blockers found and fixed

| Gap | Symptom vs other languages | Fix |
|---|---|---|
| No dict `.get(k, default)` | Verbose `map_has` + `map_get` pairs. | `(getd m k d)` sugar. |
| No min/max | Manual if/compare noise. | `(min a b)` / `(max a b)` sugar. |
| Case-insensitive equality | Repeated `to_lower` on both sides. | `(eq_ci a b)` sugar. |
| Next-id counter boilerplate | Repeated sget/sput/+1 in notes/REST. | `(snxt handle key)` sugar. |

## Apps shipped / densified

1. **`apps/eqci_cli`**, **`apps/minmax_cli`**, **`apps/getd_cli`**
2. Densified `notes_cli` (`snxt`/`blank`), `rest_tasks`/`task_api` (`snxt`/`getd`/`blank`),
   `webhook_relay` (`blank`/`switch`), `json_get` (`blank`)

## Token proof

```bash
python3 benchmarks/real_apps/prove_real_apps.py
```

## Remaining limits

- `middleware` still Unsupported
- `match` still Unsupported (prod-flip lock)
- `schema` / SQL migrations still Unsupported
