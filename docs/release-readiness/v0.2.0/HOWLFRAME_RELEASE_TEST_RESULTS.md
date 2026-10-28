# HowlFrame release test results (v0.2.0 release candidate)

Host: Linux x86-64 (Ubuntu 26.04 userland), Python 3.14, Node 22.22.1, Chromium 148 (Playwright). Go 1.25.14 for every
gate and staged build (`GOTOOLCHAIN=go1.25.14`; CI's `setup-go '1.25'` resolved to 1.25.14 on 2026-10-27). Exact
commands are given with each section; `$HF` is the binary under test.

## Commits

| Name | SHA |
|---|---|
| Base `main` | `def34540969ff7bfcff4275263215fb3e0253630` |
| Numeric fix (`fix/numeric-nonfinite`) | FINAL_NUMERIC_SHA |
| Release candidate (`release/v0.2.0-rc`) | FINAL_RC_SHA |

## 1. Engineering gates

FINAL_GATES

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

STAGED_BINARY_HASHES

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

STAGED_ACCEPTANCE

EvidenceDesk and the HowlBoard consumer (HowlBoard `main` `2e17439`, current with origin):

```bash
HOWLFRAME_BIN=<staged> bash milestone3/evidencedesk/scripts/build.sh
HOWLFRAME_BIN=<staged> HOWLBOARD_REPO=<howlboard> python3 milestone3/evidencedesk/tests/verify_workflows.py
```

STAGED_EVIDENCEDESK

## 6. Other platforms

| Target | Result |
|---|---|
| linux/arm64 staged binary under `qemu-aarch64-static` | `version` correct; numeric programs identical to amd64; build, denial and grant correct; amd64- and arm64-compiled artifacts byte-identical and interchangeable |
| darwin/amd64, darwin/arm64, windows/amd64 | Built; **not executed** (no host available) |

## 7. Compatibility

See `HOWLFRAME_RELEASE_COMPATIBILITY.md` (published v0.1.0 binary against the staged binary; malformed artifacts).

## Observations outside the release scope

* `apps/echo_api`'s Go test starts its server with `go run` and kills only the `go` process, so the compiled
  `howlframe -run-bc ... echo_api.howl.bc.bin` keeps listening on `*:8106` after `go test ./...`. Found and killed after
  a gate run. Test-only; recommended follow-up (start the built binary directly, or kill the process group).
