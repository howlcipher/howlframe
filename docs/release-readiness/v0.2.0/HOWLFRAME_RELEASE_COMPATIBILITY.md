# HowlFrame release compatibility (v0.2.0 release candidate)

Release candidate: branch `release/v0.2.0-rc` (stacked on `fix/numeric-nonfinite`). Base `main` `def34540`.

## Version decision: v0.2.0

| Fact (verified) | Source |
|---|---|
| Published release v0.1.0 (2026-08-13) reports `HowlFrame 0.1.0`, `HFBC format: 1` and writes format 1 | Published `howlframe_v0.1.0_linux_amd64.tar.gz`, checksum verified |
| `main` at `def34540` reports `HowlFrame 0.1.0`, `HFBC format: 2` and writes format 2 since PR #74 (2026-10-01) | `howlframe version`, artifact header bytes |
| v0.1.0 rejects format 2 artifacts | `unsupported artifact version: expected 1, got 2` |
| Grants narrowed after v0.1.0 (bare `process` no longer allows `exec`, coarse `network` no longer allows `fetch`, model calls need `network`, `process:env=` refusals) | `change_log.md` "Breaking"; reproduced with the staged binary |
| Numeric errors replace NaN/Inf values; out-of-range literals no longer compile | `HOWLFRAME_NUMERIC_CORRECTNESS.md` |
| `map_get` of a missing key changed from `""` to `nil` (#137) | v0.1.0 artifact run on both runtimes |
| The repository has no written versioning policy | Searched docs and contributing guides |
| A local, unpushed `v0.1.1` tag exists on an unmerged commit (`e2b62f4c`, 2026-08-13); no v0.1.1 release exists | `git tag`, `gh release list`, `git ls-remote --tags` |

Without a repository policy, Semantic Versioning's 0.x convention applies: incompatible changes raise the minor
version. The artifact format and grant changes are incompatible for users of v0.1.0, so a patch release (0.1.1) would
claim compatibility that does not exist; 0.1.1 is also the name of the abandoned local tag. 1.0.0 would claim a
stability the product directive does not. **Proposed: v0.2.0.** The version is an Owner decision.

## Version reporting after the change

| Surface | v0.1.0 | `main` `def34540` | RC |
|---|---|---|---|
| `howlframe version` / `--version` | `HowlFrame 0.1.0` / `HFBC format: 1` | `HowlFrame 0.1.0` / `HFBC format: 2` | `HowlFrame 0.2.0` / `HFBC format: 2` |
| `howlframe help` banner | `HowlFrame v0.1 CLI` | `HowlFrame v0.1 CLI` | `HowlFrame 0.2.0 CLI` (from the version constant) |
| `interop --json` `version` / `hfbc_format` | n/a | `0.1.0` / `2` | `0.2.0` / `2` |
| Header of a written artifact | `HFBC` + `01 00 00 00` | `HFBC` + `02 00 00 00` | `HFBC` + `02 00 00 00` |
| Release workflow | builds whatever the tag says | same | refuses a tag that differs from `howlframe version` |

`TestAdvertisedHFBCFormatMatchesWrittenArtifact` (existing) ties the advertised format to the written header; the CLI test
pins `HowlFrame 0.2.0` and the help banner. Neither `version` nor `inspect` reports the format of a given artifact;
it is the little-endian uint32 at bytes 4 to 8.

## HFBC compatibility, executed

Format 1 artifacts were compiled by the **published v0.1.0 binary** from v0.1.0-era sources (`git show v0.1.0:...`)
and run on both the published v0.1.0 binary and the staged v0.2.0 linux/amd64 binary, with the same arguments and
grants.

| Artifact (format 1) | v0.1.0 | v0.2.0 staged | Result |
|---|---|---|---|
| `cli_hello` | prints greeting, exit 0 | same | Same |
| `log_analyzer app.log` (filesystem) | summary, exit 1 (ATTENTION) | same | Same |
| `action_executor proposal.json` | DENY JSON | same | Same |
| Arithmetic, strings, lists, `to_int`, `to_float` of finite text (14 single-expression programs) | values | same | Same (v0.1.0 stored integer literals as floats, so these artifacts keep float semantics) |
| `release_authority proposal.json` | `"target": ""` | `"target": "unknown"` | **Differs** (`map_get` missing key now `nil`) |
| `kv_cli set theme dark get theme` (database) | `OK`, `dark` | `STACK_UNDERFLOW` at `STORE_OPEN` | **Fails closed** |
| `todo_cli add ... list` (database) | added, listed | `STACK_UNDERFLOW` at `STORE_OPEN` | **Fails closed** |
| `spawn_agent` without `task` | `VM internal panic: unknown opcode: SPAWN_AGENT` | `STACK_UNDERFLOW` at `SPAWN_AGENT` | Fails on both |
| `spawn_agent` with `(task ...)` bodies (`examples/swarm_dogfood`, found by the independent review) | `VM internal panic: unknown opcode: TASK` | Runs to `swarm done` under `process` | **Differs**: works only on v0.2.0 |
| `to_float "nan"`, `(* 1e308 10.0)`, `(/ 1 0)` | `NaN`, `+Inf`, `+Inf` | `CONVERSION_ERROR` / `RUNTIME_ERROR` | **Fails closed** (intended) |

An opcode registry comparison (v0.1.0 `internal/bytecode/opcode.go` against the RC) explains the differences: 22
opcodes were added, opcode numbers shifted (format 1 artifacts are resolved by opcode name, so this is handled),
`STORE_OPEN` now pops its URI (0 to 1 pops), `SPAWN_AGENT` gained an operand, and `CONFIDENCE`,
`NEURAL_CIRCUIT`, `EPHEMERAL_CIRCUIT` now require `network` while `RES`/`RES_JSON` no longer do.

The previous `artifact_compatibility.md` said existing format 1 artifacts "will execute normally". That was false
for `store_open` and `map_get`. Rejecting format 1 entirely was considered and not done: HowlPlane's Mission007
deliberately ships a portable format 1 artifact verified on both runtimes
(`docs/journals/2026-10-21_mission007_frame_compatibility.md`). The document now states the measured behavior and
recommends recompiling.

| Other checks | Result |
|---|---|
| Format 2 artifact (staged) on v0.1.0 | `unsupported artifact version: expected 1, got 2`, exit 1 |
| Format 2 artifact on v0.2.0 | Runs |
| Same source compiled by the amd64 and arm64 staged binaries | Byte-identical artifacts |
| Bad magic, random bytes, JSON, version 0 or 3, truncated, header only, empty, flipped payload byte, trailing byte, length over 10 MiB | Exit 1 with one specific message on `run`, `-run-bc` and `inspect --caps`; no panic output |
| Artifact holding a `+Inf` constant (written by the base compiler) | Refused at load on v0.2.0; the base runtime printed `+Inf` |
| `inspect --caps` on format 1 and format 2 artifacts | Reports capabilities for both |

## Packaging

| Check | Result |
|---|---|
| Release workflow replicated locally (`git archive` of the RC commit, Go 1.25.14 = CI's `setup-go '1.25'` on 2026-10-27) | 5 archives + `SHA256SUMS`; `sha256sum -c` OK |
| Archive names | `howlframe_v0.2.0_{linux_amd64,linux_arm64,darwin_amd64,darwin_arm64}.tar.gz`, `howlframe_v0.2.0_windows_amd64.zip` (same scheme as the published v0.1.0 assets) |
| Archive contents | One file each: `howlframe` (mode 0755) or `howlframe.exe` |
| Executable formats | ELF x86-64, ELF aarch64, Mach-O x86-64, Mach-O arm64, PE |
| Release notes required by the workflow | `docs/releases/v0.2.0.md` present |
| Tag/version guard | `v0.2.0` against `HowlFrame 0.2.0`: passes |
| Reproducibility | With `-trimpath`, two builds in different directories give identical binaries on all five targets; without it the linux/amd64 hashes differ. Archive hashes differ between runs (tar/zip timestamps), binaries do not. |
| v0.1.0 installation guide | Its URLs (`howlframe_0.1.0_linux_amd64.tar.gz`) omitted the `v` and do not match the published assets; corrected |

Staged binary SHA-256 (from the RC commit, Go 1.25.14, `-trimpath -ldflags="-w -s"`, `CGO_ENABLED=0`) are recorded in
`HOWLFRAME_RELEASE_TEST_RESULTS.md`; a CI build of the same commit with the same Go patch release should match them.

## Platform verification

| Target | Built | Executed | How |
|---|---|---|---|
| linux/amd64 | Yes | Yes | Native: clean install per `docs/installation.md`, CLI workflows with no Go on `PATH`, Milestone 1 and 2 acceptance, EvidenceDesk, compatibility matrix |
| linux/arm64 | Yes | Under emulation only | `qemu-aarch64-static` user mode: version, numeric programs, build/run, denial and grant, cross-architecture artifacts. Not run on ARM hardware. |
| darwin/amd64, darwin/arm64 | Yes | **No** | No macOS host available. Format verified only (Mach-O headers). |
| windows/amd64 | Yes | **No** | No Windows host or Wine available. Format verified only (PE header). |

macOS and Windows binaries have never been executed in any recorded evidence, for v0.1.0 or this candidate.
