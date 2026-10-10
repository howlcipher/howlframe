# Dogfood common apps round 3 (2026-10-10)

## Goal

Further refine HowlFrame for common apps: new density sugar, widen collection
length, allow HTTP startup store init, ship three more apps, prove tokens.

## Blockers found and fixed

| Gap | Symptom vs other languages | Fix |
|---|---|---|
| `list_len` list-only | Cannot count file bytes/strings; Python `len` works on both. | Runtime + checker accept list, string, or bytes; opcode note updated. |
| Verbose string building | `str_join`/`to_string` tax vs f-strings / `fmt`. | AST sugar `(str a b…)` → `str_join` of `to_string` parts. |
| Nil default boilerplate | `(if (is_nil x) d x)` repeats in store apps. | AST sugar `(nz x d)`. |
| No `http_server` startup forms | Cannot open store once before routes; Python/Go init globals. | Checker allows `store_open` / `store_put` / `do` / `let` as startup children (bytecode already compiled them). |

## Apps shipped

1. **`apps/wc_cli`** — lines/words/bytes
2. **`apps/json_get`** — top-level JSON key lookup
3. **`apps/counter_api`** — GET/POST counter with startup store

Also densified `notes_cli` (`nz`) and `grep_cli` (`str`) in place.

## Token proof

```bash
python3 benchmarks/real_apps/prove_real_apps.py
```

| App | HowlFrame | Python | Go | Node |
|---|---|---|---|---|
| log_analyzer | 279 | 308 | 331 | 310 |
| kv_cli | 236 | 270 | 268 | 263 |
| status_api | 159 | 271 | 279 | 221 |
| notes_cli | 331 | 412 | 462 | 395 |
| auth_api | 199 | 330 | 337 | 296 |
| grep_cli | 156 | 188 | 259 | 225 |
| wc_cli | 157 | 198 | 197 | 191 |
| json_get | 161 | 185 | 189 | 161 |
| counter_api | 229 | 233 | 320 | 235 |
| **TOTAL** | **1907** | 2395 | 2642 | 2297 |

## Remaining limits

- `middleware` still Unsupported
- `match` still Unsupported (`switch`/`cond` cover dispatch)
- `schema` / SQL migrations still Unsupported
- `fetch` still body-bytes only (`http_request` for status+body)
- `map_get` miss is `""` (empty string values look missing without `map_keys`)
