# HowlFrame release test results (v0.2.0 release candidate)

Host: Linux x86-64 (Ubuntu 26.04 userland), Python 3.14, Node 22.22.1, Chromium 148 (Playwright). Go 1.25.14 for every
gate and staged build (`GOTOOLCHAIN=go1.25.14`; CI's `setup-go '1.25'` resolved to 1.25.14 on 2026-10-27). Exact
commands are given with each section; `$HF` is the binary under test.

## Commits

| Name | SHA |
|---|---|
| Base `main` | `def34540969ff7bfcff4275263215fb3e0253630` |
| Numeric fix (`fix/numeric-nonfinite`) | `2cb71aa8d0a189ba2a7a73eb260cdb77965dcf0e` (code), `9368676a` (adds its report) |
| Release candidate (`release/v0.2.0-rc`), final regression | `29c6108c9b379f9c3bba9229cf012e02e2eacb6f` |

Commits after `29c6108c` on the RC branch change only files under `docs/` (`git diff --stat 29c6108c..HEAD`).

## 1. Engineering gates

Final regression on `29c6108c` (clean tree), Go 1.25.14, `HOWL_FORBID_LOCAL_INFERENCE=1`:

| Command | Result |
|---|---|
| `test -z "$(gofmt -l .)"` | Clean |
| `go build ./...` | OK |
| `go vet ./...` | OK |
| `go test -count=1 ./...` | 76 packages ok, 0 failures |
| `go test -race -count=1 ./...` | 76 packages ok, 0 failures |
| `scripts/ci_acceptance.sh` (builds its own binary) | All suites passed: 318 PASS, 0 FAIL, 1 XFAIL (recorded waiver W-m12) |
| `python3 scripts/test_seo.py` | Passed |

The numeric branch passed the same set on `2cb71aa8` before the RC commits (76/76 test, 76/76 race, acceptance
318/0/1).

## 2. Numeric regressions

See `HOWLFRAME_NUMERIC_CORRECTNESS.md` for the reproductions. Targeted runs:

```bash
go test ./internal/vm -run 'TestFloatArithmetic|TestNaNOrdering|TestConfidenceRefuses' -count=1
go test ./internal/bytecode -run TestArtifactRefusesNonFiniteConstants -count=1
go test ./internal/backend/javascript -count=1
go test . -run 'TestNumeric|TestIssue151|TestIssue158|TestIssue161|TestIssue156|TestIssue149|TestIssue225' -count=1
```

All pass on the RC. On the base, the new tests fail (overflow returned `+Inf`, policy programs printed `ACCEPT`,
`check` accepted out-of-range literals, `confidence` returned NaN).

## 3. Staged release build

```bash
stage_release.sh <repo> <RC commit> v0.2.0 <outdir>   # replicates release.yml: version guard, 5 cross-builds,
                                                      # archives, notes check, SHA256SUMS
```

Built twice in different directories. Binary SHA-256 (identical in both builds):

```text
af9216894e9a14148b8592fc12aacc6e1d6c6115033799a6c99882a0b223bbb7  howlframe (linux/amd64)
187b7961a1cac502d813a9886d8f30ebca9c04d4a6c647af0736054d2eda5c65  howlframe (linux/arm64)
407295b27f7d59bbf4eed315f601d893e7b503571c5540cc2cc327ffc8bcb11d  howlframe (darwin/amd64)
7036019ff425555a64fd6387bdb68d40ac224114c7ac685334b79f35785b97b9  howlframe (darwin/arm64)
fc4b9c9037d52cc74804c2ac58c585912bdbeadeae06f8a5a8c86aa5af20721c  howlframe.exe (windows/amd64)
```

Archive `SHA256SUMS` of the final staging (archive hashes vary between runs because tar and zip record
timestamps; the binaries inside do not):

```text
bc0d90b1d8015fa0622cb3fa8876a3786df0a0ef8c683f69c9b502d81cdd9b8c  howlframe_v0.2.0_darwin_amd64.tar.gz
d374bcc2019039075f5d774f8d608af9d8f88830c728a912bb3ca406ec30215e  howlframe_v0.2.0_darwin_arm64.tar.gz
d97e6a501adf80ce973c641a32b7e0ce1e307c7ad314216512b0afce573a3bd4  howlframe_v0.2.0_linux_amd64.tar.gz
47728202b72f0e0120c1a2847c787a00a9c73db1839cfcbab2f471e510f53163  howlframe_v0.2.0_linux_arm64.tar.gz
0e7987b0c2568d58de8ec30747aeda94eb92077b5d692744e18194b1178450ad  howlframe_v0.2.0_windows_amd64.zip
```

The final staging from `29c6108c` produced the same five binary hashes as the staging from `bbd4c8f5` that the
independent reviewer and the acceptance runs used.

`sha256sum -c SHA256SUMS`: all five archives OK. Each archive holds one 0755 executable; formats ELF x86-64, ELF
aarch64, Mach-O x86-64, Mach-O arm64, PE.

## 4. Clean installation (staged linux/amd64 archive)

In an empty directory with `env -i HOME=<empty> PATH=<install prefix>:<curated tools without go>`:

```bash
VERSION=v0.2.0
sha256sum --check --ignore-missing SHA256SUMS          # OK
tar -xzf howlframe_${VERSION}_linux_amd64.tar.gz
install -m 0755 howlframe <prefix>/bin/howlframe
howlframe version                                      # HowlFrame 0.2.0 / HFBC format: 2
```

| Command | Result |
|---|---|
| `check hello.howl` / `build hello.howl` / `run hello.hfbc` / `run hello.howl` | `OK`, `Built hello.hfbc`, greeting x2; exit 0 |
| `check bad.howl` (`(+ 1 "two")`), `build bad.howl` | JSON diagnostic with line and column; exit 1 |
| `check syntax.howl` (unterminated string) | `Unterminated string` at 1:17; exit 1 |
| `inspect --caps hello.hfbc`, `inspect --caps writer.hfbc` | `[]`; `["filesystem"]` with the `WRITE_FILE` site |
| `run writer.hfbc` / `run --allow-caps filesystem:write=. writer.hfbc` | `CAPABILITY_DENIED` exit 1 / `wrote` exit 0 |
| `run spin.howl` (infinite loop), `--max-instructions 5000`, `--deadline 300ms` | `LIMIT_EXCEEDED` (instruction limit, instruction limit, wall-clock deadline); exit 1 |
| `check --profile governed writer.howl` | `OK` |
| `run missing.hfbc` | `Cannot read file`; exit 1 |

No Go toolchain was on `PATH`.

## 5. Application acceptance with the staged binary

From a `git archive` export of the RC commit, `HF_BIN=<staged binary>`, no Go on `PATH`:

```bash
scripts/ci_acceptance.sh
```

| Suite | Result |
|---|---|
| LedgerLens frozen acceptance | 35 / 35 |
| HealthWatch frozen acceptance | 29 / 29 |
| TaskFlow frozen acceptance | 32 / 32 |
| Milestone 1 retained regressions (fresh boundaries, HealthWatch supplemental, LedgerLens lowest-line) | Pass |
| TicketDesk frozen acceptance | 101 / 101 |
| TicketDesk developer-experience checks | 13 / 13 |
| TicketDesk reassessment / adversarial | 11 / 11 |
| TicketDesk historical review regressions | 10 / 10 |
| Assurance m10 | 5 tests, 0 failed, 1 expected failure (waiver W-m12, recorded) |
| Restart at default ceilings (1,000 tickets, 100 tokens), SIGKILL, invalid state | 3 / 3 |
| Total | 318 PASS, 0 FAIL, 1 XFAIL; exit 0 |

EvidenceDesk and the HowlBoard consumer (HowlBoard `main` `2e17439`, current with origin):

```bash
HOWLFRAME_BIN=<staged> bash milestone3/evidencedesk/scripts/build.sh
HOWLFRAME_BIN=<staged> HOWLBOARD_REPO=<howlboard> python3 milestone3/evidencedesk/tests/verify_workflows.py
```

| Check | Result |
|---|---|
| `build.sh` with the staged compiler | OK |
| Live loopback listener on `127.0.0.1` only | PASS |
| Exact restart persistence, encoded IDs, Unicode notes, physical-line diagnostics | PASS |
| Real Chromium 148: import, header, filter, ordered claims, notes, reload, errors, XSS, keyboard | PASS |
| Actual HowlBoard consumer, `review.jsonl` (3 records, 2 tasks) and `ledger_sample.jsonl` (72 records, 16 tasks) | PASS |
| Denied network, environment, database, `filesystem:read`, `filesystem:write`, static read (no persisted corruption) | PASS |
| Too-small instruction limit; process and remote-fetch grants absent | PASS |

Only the committed sanitized fixtures were used.

## 6. Other platforms

| Target | Result |
|---|---|
| linux/arm64 staged binary under `qemu-aarch64-static` | `version` correct; numeric programs identical to amd64; build, denial and grant correct; amd64- and arm64-compiled artifacts byte-identical and interchangeable |
| darwin/amd64, darwin/arm64, windows/amd64 | Built; **not executed** (no host available) |

## 7. Compatibility

See `HOWLFRAME_RELEASE_COMPATIBILITY.md` (published v0.1.0 binary against the staged binary; malformed artifacts).

## Observations outside the release scope

* `apps/echo_api`'s Go test starts its server with `go run` and kills only the `go` process, so the compiled
  `howlframe -run-bc ... echo_api.howl.bc.bin` keeps listening on `*:8106` after `go test ./...`. Found after two gate runs (killed each time). Test-only; recommended follow-up (start the built binary directly, or kill the process group).
