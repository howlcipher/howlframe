# Dogfood common apps round 2 (2026-10-10)

## Goal

Ship more common app shapes on HowlFrame alone, fix remaining blockers, and prove
token density against idiomatic Python/Go/Node ports.

## Blockers found and fixed

| Gap | Symptom vs other languages | Fix |
|---|---|---|
| `store_open` URI literal-only | Cannot open `file://` path from `env` / computed URI; Python/Go open paths from variables. | Compile URI as expression; pop at runtime; keep `StringOperand2` static hint for `RequiredCapabilities`. |
| No pattern-match sugar | Nested `if`/`cond` for CLI verbs and auth headers is token-heavy vs Go `switch` / Python match. | AST sugar `(switch expr (pat body…)… (else body…))` → `let`+`cond`; `(inc x)` → `(set x (+ x 1))`. |
| Auth without middleware | `middleware` still Unsupported; other langs use frameworks. | Hand-rolled bearer check with `switch`/`req_header` in route lambdas (`apps/auth_api`). |

## Apps shipped

1. **`apps/notes_cli`** — persistent add/list/get/delete via file store + `NOTES_STORE`
2. **`apps/auth_api`** — `/health`, bearer `/secret`, authenticated `/echo/{name}`
3. **`apps/grep_cli`** — regex line grep with `hits=` and capability denial

## Token proof (suite includes prior densified apps)

```bash
python3 benchmarks/real_apps/prove_real_apps.py
```

Harness requires app tests + cross-lang parity + HowlFrame under Python per app
and under every language on the total (`cl100k_base`).

### Measured totals

| App | HowlFrame | Python | Go | Node |
|---|---|---|---|---|
| log_analyzer | 279 | 308 | 331 | 310 |
| kv_cli | 236 | 270 | 268 | 263 |
| status_api | 159 | 271 | 279 | 221 |
| notes_cli | 343 | 412 | 462 | 395 |
| auth_api | 199 | 330 | 337 | 296 |
| grep_cli | 171 | 188 | 259 | 225 |
| **TOTAL** | **1387** | 1779 | 1936 | 1710 |

## Remaining limits (not blocking these apps)

- `middleware` still Unsupported (auth stays in-handler)
- `match` still Unsupported (`switch`/`cond` cover dispatch)
- `schema` / SQL migrations still Unsupported
- `fetch` still body-bytes only (`http_request` for status+body)
- Round 3 continues in `docs/journals/2026-10-10_dogfood_apps_round3.md`
