# HowlFrame v0.2.1 and v0.3.0 publication record

**Status: PUBLISHED AND VERIFIED.** Both releases were published on 2026-11-25 with the Owner's approval, at the
commits prepared on 2026-11-23 (`docs/journals/2026-11-23_vm_spawn_and_input_bounds.md`). v0.2.1 was pushed first
and v0.3.0 second, so v0.3.0 is the GitHub "Latest" release. Every check below ran against the files users download.

## Publication

| | v0.2.1 | v0.3.0 |
|---|---|---|
| Commit | `45008d30307f08a7352914d9db40577a79f104f5` (branch `claude/howlframe-v0.2.1-patch`: `v0.2.0` + `2bce76f0` + release prep) | `7318762322d659a3065d164a8c7eb7508b056877` (merge of #315 on `main`) |
| Tag | annotated, SSH-signed, tag object `4564f8ad6f87b4cb612acf3f8f40e095da8e9b13`, message `HowlFrame v0.2.1` | annotated, SSH-signed, tag object `6f36be770cd33273e2449a681e71337badbb28c6`, message `HowlFrame v0.3.0` |
| Release | https://github.com/howlcipher/howlframe/releases/tag/v0.2.1 (2026-11-25T19:06:03Z) | https://github.com/howlcipher/howlframe/releases/tag/v0.3.0 (2026-11-25T19:07:39Z, Latest) |
| Workflow | https://github.com/howlcipher/howlframe/actions/runs/45681665294: five `Build` jobs and `Publish Release` succeeded | https://github.com/howlcipher/howlframe/actions/runs/45681925015: five `Build` jobs and `Publish Release` succeeded; Go 1.25.14 |
| Release notes | `docs/releases/v0.2.1.md` (on the v0.2.1 tag only) | `docs/releases/v0.3.0.md` |

Neither release is a draft or prerelease.

## Preflight

- **v0.2.1:** CI had never run on its branch, so it was verified locally on a detached checkout of `45008d30` with Go 1.26.0:
  - `go vet ./...` was clean.
  - `go build` succeeded, and `howlframe version` printed `HowlFrame 0.2.1` and `HFBC format: 2`.
  - `go test -p 1 ./internal/vm/` passed.
  - `go test -race -count=3` on the spawn, read_line, parse_json and input-memory tests passed.
  - The full `go test -p 1 ./...` exited 0, with 76 packages `ok` and no `FAIL`.
- **v0.3.0:** CI on `main` at `73187623` was green. A local build printed `HowlFrame 0.3.0` and `HFBC format: 2`.

## Published assets

Downloaded with `gh release download` into empty directories. `sha256sum -c SHA256SUMS` passed for all five files of each release.

```
8490a2e28fe4261d2d2d8177d8c241501071e58e9e2a719016fdaa3749bba435  howlframe_v0.2.1_darwin_amd64.tar.gz
21900b60373c19f029d4b7bb916e07215ff863c44ba765868ec64a2e24aeebd6  howlframe_v0.2.1_darwin_arm64.tar.gz
cf2235ca738c483cfe5099e110e851074da14650a850374d71e3b0a5df177c0f  howlframe_v0.2.1_linux_amd64.tar.gz
b7f80e1888436aeec029a58f7b964f48d1a3ed9f263e3ce66e70cd1516d74e5e  howlframe_v0.2.1_linux_arm64.tar.gz
4ea703304193f85a3e568dceeac2d2fb3dfa8e54a78c03e6b665f2006003f297  howlframe_v0.2.1_windows_amd64.zip

1e4e1a73bf67ccff4d563630a91a8a6b2ff5de98465f0e3e5e557f1b073d2fe5  howlframe_v0.3.0_darwin_amd64.tar.gz
e0ee9ad4bb7c5d6db15cccfca8ce46875e21bd40beac114d6f397fc557838de6  howlframe_v0.3.0_darwin_arm64.tar.gz
d9fa6d42127439f0fad4dceaedf996bf84603d748ccb7559a5fa8a1e795ddce3  howlframe_v0.3.0_linux_amd64.tar.gz
6d8af0378df072517e48c090843970c9d740e89420a6717038d6fcaf87d3f79a  howlframe_v0.3.0_linux_arm64.tar.gz
3be98e7f0233b3f6877acd7cc1ef87c05132e364e494ab4842dce4faff1ba13d  howlframe_v0.3.0_windows_amd64.zip
```

The published v0.2.1 linux/amd64 executable (`7f0cc98e…`) printed `HowlFrame 0.2.1` and `HFBC format: 2`.

## Clean installation and standalone program (Linux x86-64, v0.3.0)

`clean_install_smoke.sh` in this directory is the v0.2.0 script with the version strings changed to v0.3.0. It was
run under the following conditions:

- `env -i`, with a new empty `HOME`.
- A `PATH` holding the install directory and links to every `/usr/bin` tool except `go` and `gofmt`.
- No development binary on the `PATH`.

Result: all 24 checks passed. The installed executable hash was
`c8f27a1c982657b792cdd3672c003b15024cd94d217e38faac4f9a32ff4f2fac`. The checks cover these areas:

- version, help and interop
- check, build and run of a standalone program
- rejection of invalid source
- filesystem grant scoping
- `process:exec=` scoping
- the instruction budget

A first run had the full `/usr/bin` on its `PATH`. On this host that includes Go, so the `no go on PATH` check
failed. That was the environment, not the release. The results above are from the rerun without Go.

## Platform status

| Platform | Status |
|---|---|
| Linux x86-64 | Published v0.3.0 binary executed (clean install, this record); published v0.2.1 binary ran `version` |
| Linux ARM64, macOS x86-64, macOS ARM64, Windows x86-64 | Cross-compiled by the release workflow, **not executed** for these releases (#238) |

## Limitations shipped

These are unchanged from v0.2.0 except as the release notes state:

- #216 and #217: exec containment.
- #182: the `process:env=` deny-list.
- #237: generated Go numeric checks.
- #236: exit status for invalid resource limits.
- #240: unsigned binaries and artifacts. The tags are signed; the archives are not.

`terminal_app` (v0.3.0) runs on the bytecode VM only.
