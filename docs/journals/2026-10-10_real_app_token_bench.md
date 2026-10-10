# Real-app token bench: density without functionality loss (2026-10-10)

## Goal

Prove HowlFrame token wins on dogfooded applications (not toy snippets), with
existing app tests and cross-language parity as the no-behavior-loss gate.

## What shipped

1. **Density sugar** in `internal/ast/desugar.go` (CompileTimeOnly registry entries):
   - `(cond …)` → nested `if`
   - multi-binding `(let (a x b y) …)` → nested single-binding `let`
   - `(sget` / `sput` / `sinc)` for the common store string-`val` record shape
2. **In-place densify** of `apps/log_analyzer`, `apps/kv_cli`, `apps/status_api`
   (existing `go test ./apps/...` contracts unchanged).
3. **Cross-language ports** under `benchmarks/real_apps/{app}/{python,go,node}/`
   implementing the same CLI/HTTP contracts.
4. **Proof harness** `benchmarks/real_apps/prove_real_apps.py` — fails unless
   app tests pass, parity passes for every language, and HowlFrame beats Python
   per app and every language on the total (`cl100k_base`).
5. **CI gate** in `.github/workflows/ci.yml`.

## Evidence

| App | HowlFrame | Python | Go | Node |
|---|---|---|---|---|
| log_analyzer | **279** | 308 | 331 | 310 |
| kv_cli | **236** | 270 | 268 | 263 |
| status_api | **159** | 271 | 279 | 221 |
| **TOTAL** | **674** | 849 | 878 | 794 |

Relative to Python total: **−175 tokens (−20.6%)** with parity + app tests green.

Verified:

- `go test ./apps/kv_cli/ ./apps/log_analyzer/ ./apps/status_api/`
- `go test ./internal/ast/ ./internal/construct/ ./internal/bytecode/`
- `python3 benchmarks/real_apps/prove_real_apps.py`
- `tests/test_real_app_density.howl`

## Notes

Ports are idiomatic equivalents of the behavioral contracts (CLI I/O and HTTP
JSON), not line-for-line API translations. HowlFrame retains capability-gated
`store_*` / filesystem / network surfaces; density sugar collapses ceremony
without changing runtime opcodes.
