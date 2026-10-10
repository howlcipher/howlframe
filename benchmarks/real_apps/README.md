# Real-app token + functionality benchmark

This suite proves HowlFrame density wins on **dogfooded applications**, not toy snippets.

HowlFrame sources are the live apps in `apps/`. Python/Go/Node ports live here.

## Prove it

```bash
go build -o howlframe howlframe.go
python3 benchmarks/real_apps/prove_real_apps.py
```

### Latest measured (`cl100k_base`)

| App | HowlFrame | Python | Go | Node |
|---|---|---|---|---|
| log_analyzer | **279** | 308 | 331 | 310 |
| kv_cli | **236** | 270 | 268 | 263 |
| status_api | **159** | 271 | 279 | 221 |
| notes_cli | **331** | 412 | 462 | 395 |
| auth_api | **199** | 330 | 337 | 296 |
| grep_cli | **156** | 188 | 259 | 225 |
| wc_cli | **157** | 198 | 197 | 191 |
| json_get | **157** | 185 | 189 | 161 |
| counter_api | **229** | 233 | 320 | 235 |
| fetch_cli | **109** | 185 | 199 | 214 |
| template_cli | **160** | 206 | 190 | 203 |
| stamp_api | **135** | 239 | 264 | 213 |
| mkdir_cli | **50** | 90 | 114 | 92 |
| contains_cli | **130** | 158 | 148 | 156 |
| file_transform | **229** | 266 | 293 | 238 |
| **TOTAL** | **2716** | 3539 | 3850 | 3413 |

Latest proof artifact: [`proof_latest.json`](proof_latest.json).
