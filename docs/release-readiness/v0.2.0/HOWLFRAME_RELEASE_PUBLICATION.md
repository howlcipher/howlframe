# HowlFrame v0.2.0 publication record

**Status: PUBLISHED AND VERIFIED.** Published on 2026-10-28 with the Owner's approval of v0.2.0 at the exact commit
below, as an experimental release with the limitations in `SECURITY.md` and the release notes. Every check in this
record was run against the files users download, not against a development build.

## Publication

| | |
|---|---|
| Approved commit | `77cb55c87076aef846a601f18ace6bba78a12010` (merge of #234; #233 merged as `2685f945`) |
| Tag | `v0.2.0`, annotated, tag object `57363ea72b6f9e14b84b0e1693572e08aad14844`, message `HowlFrame v0.2.0` |
| Release | https://github.com/howlcipher/howlframe/releases/tag/v0.2.0 (published 2026-10-28T18:35:10Z, not a draft or prerelease) |
| Workflow | https://github.com/howlcipher/howlframe/actions/runs/40746345066: five `Build` jobs and `Publish Release` succeeded; Go 1.25.14 |
| Release notes | `docs/releases/v0.2.0.md`, used as the release body by the workflow |

Preflight, run immediately before tagging: `origin/main` was the approved commit; #233 and #234 merged; `build`,
`acceptance`, `race` and `latest-go` passed on the commit; no remote `v0.2.0` tag or release existed; the workflow
contains the tag/version check; `go run howlframe.go version` printed `HowlFrame 0.2.0` and `HFBC format: 2`; the
release notes, `SECURITY.md` and `docs/installation.md` carry the platform and security statements; the live project
page says "not an OS sandbox". Only `refs/tags/v0.2.0` was pushed; the never-pushed local `v0.1.1` tag was not.

## Published assets

Downloaded from the public release URLs into an empty directory. `sha256sum -c SHA256SUMS`: all five OK.

```
c88caf30767110409196c4ce36583f136e5c32e1065d70ea694f6faae6c968f9  howlframe_v0.2.0_darwin_amd64.tar.gz
e592637a84df248d511e0c394497b209d3adaa04b37d786ad02301c4ab786d9d  howlframe_v0.2.0_darwin_arm64.tar.gz
692c7130588422240eb84ee9682c26efc2d0fb9b0f1519bcc7c84b45bff83205  howlframe_v0.2.0_linux_amd64.tar.gz
e64e3131165c279efe56be72547aec234bbe13bce34d9bad2e0c0b90e82714e4  howlframe_v0.2.0_linux_arm64.tar.gz
d3bf3dce3e5d7685669f7a63d29f4148792832e7557434d171f6f7dae0df1ba4  howlframe_v0.2.0_windows_amd64.zip
```

Each archive holds one 0755 executable (`howlframe`, or `howlframe.exe`). The archive hashes differ from the staging
run because tar and zip record timestamps; the executables were compared separately and all five are byte-identical
to the staged binaries in `HOWLFRAME_RELEASE_TEST_RESULTS.md`:

| Target | Executable SHA-256 | Format | Staged |
|---|---|---|---|
| linux/amd64 | `af9216894e9a14148b8592fc12aacc6e1d6c6115033799a6c99882a0b223bbb7` | ELF x86-64 | Match |
| linux/arm64 | `187b7961a1cac502d813a9886d8f30ebca9c04d4a6c647af0736054d2eda5c65` | ELF aarch64 | Match |
| darwin/amd64 | `407295b27f7d59bbf4eed315f601d893e7b503571c5540cc2cc327ffc8bcb11d` | Mach-O x86-64 | Match |
| darwin/arm64 | `7036019ff425555a64fd6387bdb68d40ac224114c7ac685334b79f35785b97b9` | Mach-O arm64 | Match |
| windows/amd64 | `fc4b9c9037d52cc74804c2ac58c585912bdbeadeae06f8a5a8c86aa5af20721c` | PE (MZ) | Match |

The staged binaries were built from `29c6108c`; the commits after it change only files under
`docs/release-readiness/`, which the binary does not embed. A local rebuild of the release commit with Go 1.25.14 and
the workflow's flags also gave `af921689…` for linux/amd64.

## Clean installation and standalone program (Linux x86-64)

`clean_install_smoke.sh` (in this directory) ran with `env -i`, a new empty `HOME`, and a `PATH` holding only the
install directory and a few shell tools: no Go toolchain and no development binary. It follows the Linux section of
`docs/installation.md` (installing to `~/bin` instead of `sudo install` to `/usr/local/bin`). Result: 26 of 26 checks
passed.

| Check | Observed |
|---|---|
| Download and `sha256sum --check --ignore-missing SHA256SUMS` | OK; installed executable `af921689…` |
| `howlframe version` | `HowlFrame 0.2.0`, `HFBC format: 2` |
| `howlframe help` | starts `HowlFrame 0.2.0 CLI`, lists the commands |
| `howlframe interop --json` | `schema` `howl.interop/v1`, `version` `0.2.0`, `hfbc_format` `2` |
| `check`, `build`, `run` of a hello program | `OK`, `Built hello.hfbc` (magic `HFBC`, format 2), prints the greeting |
| Type error, unbalanced parenthesis | exit 1 with reason, line and column; `build` writes no artifact |
| `read_file` with no grant / with `filesystem:read=<dir>` / outside the root | `CAPABILITY_DENIED` / reads the file / `CAPABILITY_DENIED` |
| `exec` with no grant / bare `process` / `process:exec=<canonical path>` | `CAPABILITY_DENIED: process` / `exec argv is not in the allow-list` / runs |
| Endless loop with `--max-instructions 10000` | `LIMIT_EXCEEDED: instruction limit exceeded`, exit 1 (not the 60 s timeout) |

## Artifact compatibility (published binaries)

Format 1 artifacts compiled by the published v0.1.0 binary from v0.1.0 sources (`git show v0.1.0:...`), run on the
published v0.1.0 and v0.2.0 binaries with the grants the v0.1.0 tests use:

| Artifact | v0.1.0 | v0.2.0 | Matches the documented policy |
|---|---|---|---|
| `examples/cli_hello` | `Hello, World!` | same | Yes (same) |
| `apps/kv_cli set theme dark get theme` (`database`) | `OK`, `dark` | `STACK_UNDERFLOW` at `STORE_OPEN` | Yes (fails closed) |
| `apps/release_authority` with a proposal lacking `target` (`filesystem,database`) | `"target": ""` | `"target": "unknown"` | Yes (`map_get` of a missing key is `nil`) |
| v0.2.0 `hello.hfbc` (format 2) on v0.1.0 | `unsupported artifact version: expected 1, got 2`, exit 1 | runs | Yes |

Loading format 1 does not promise identical behavior; `docs/reference/artifact_compatibility.md` says so and
recommends recompiling.

## Platform status

| Platform | Status |
|---|---|
| Linux x86-64 | Published binary executed: clean install, standalone program, compatibility (this record); acceptance suites on the byte-identical staged binary |
| Linux ARM64 | Published binary executed under qemu user-mode emulation only: `version`, `check`, `build`, `run`; its artifact is byte-identical to the x86-64 one |
| macOS x86-64, macOS ARM64 | Cross-compiled, **not executed** (no macOS host) |
| Windows x86-64 | Cross-compiled, **not executed** (no Windows host) |

## Limitations shipped

Unchanged from the release candidate and accepted by the Owner for this experimental release: #216 (a hostile program
can defeat the exec supervisor), #217 (weaker descendant containment on macOS, BSD and Windows), #182
(`process:env=` deny-list), generated Go numeric semantics, VM budgets are not OS resource limits, inconsistent exit
status for invalid resource limits, format 1 artifacts may behave differently, unsigned binaries and artifacts,
macOS and Windows unexecuted.

## Observations (not failures)

* A read outside a `filesystem:read=` root reports `capability denied: filesystem`, the same text as having no grant;
  the message does not say which root was granted or that the path fell outside it.
* Runtime errors carry the function and instruction index, not the source line.
* `(print (read_file ...))` prints a list of byte values; `bytes_to_string` is needed. This is the documented native
  bytes behavior (#230), but it surprised the first smoke program.

No check failed against the published release. The first smoke run had script errors (a missing `tee`, a shell alias
for `grep`, `print` of bytes, and `/usr/bin/id` being a symlink, which the runtime correctly refused); the script was
corrected and rerun from an empty `HOME`, and the results above are from that run.
