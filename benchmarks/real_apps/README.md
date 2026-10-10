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
| json_get | **146** | 185 | 189 | 161 |
| counter_api | **229** | 233 | 320 | 235 |
| fetch_cli | **100** | 185 | 199 | 214 |
| template_cli | **153** | 206 | 190 | 203 |
| stamp_api | **128** | 239 | 264 | 213 |
| mkdir_cli | **51** | 90 | 114 | 92 |
| contains_cli | **119** | 158 | 148 | 156 |
| file_transform | **229** | 266 | 293 | 238 |
| env_cli | **81** | 105 | 103 | 104 |
| starts_cli | **118** | 157 | 147 | 155 |
| ends_cli | **118** | 157 | 148 | 155 |
| head_cli | **139** | 168 | 192 | 177 |
| cut_cli | **156** | 173 | 209 | 180 |
| has_line_cli | **123** | 162 | 164 | 161 |
| join_cli | **98** | 139 | 127 | 132 |
| uniq_cli | **127** | 153 | 182 | 162 |
| echo_api | **109** | 284 | 219 | 223 |
| now_cli | **15** | 41 | 24 | 18 |
| sleep_cli | **58** | 94 | 126 | 103 |
| **TOTAL** | **3814** | 5172 | 5491 | 4983 |

Latest proof artifact: [`proof_latest.json`](proof_latest.json).
