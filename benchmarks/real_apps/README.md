# Real-app token + functionality benchmark

This suite proves HowlFrame density wins on **dogfooded applications**, not toy snippets.

## Apps under test

| App | Contract |
|---|---|
| `log_analyzer` | CLI file scan, regex counters, missing-file + capability denial |
| `kv_cli` | In-memory store CLI: set/get/delete/increment sequences |
| `status_api` | HTTP health/ready/version/config with env-gated readiness |

HowlFrame sources are the live apps in `apps/` (densified in place). Python/Go/Node ports live here and implement the same behavioral contracts exercised by `apps/*_test.go`.

## Prove it

```bash
go build -o howlframe howlframe.go
python3 benchmarks/real_apps/prove_real_apps.py
```

The harness fails unless:

1. `go test` for each densified HowlFrame app still passes (no functionality loss)
2. Parity scenarios pass for HowlFrame, Python, Go, and Node
3. HowlFrame token counts (`cl100k_base`) beat Python per app and beat every language on the total

### Latest measured (`cl100k_base`)

| App | HowlFrame | Python | Go | Node |
|---|---|---|---|---|
| log_analyzer | **279** | 308 | 331 | 310 |
| kv_cli | **236** | 270 | 268 | 263 |
| status_api | **159** | 271 | 279 | 221 |
| **TOTAL** | **674** | 849 | 878 | 794 |

Latest proof artifact: [`proof_latest.json`](proof_latest.json).
