# HowlFrame Bytecode Artifact Format & Compatibility

## Overview

HowlFrame bytecode programs (`.hfbc`) are compiled binary representations of parsed and checked Howl programs.

To guarantee cryptographic provenance and deterministic builds, HowlFrame bytecode artifacts must serialize deterministically: identical source code compiled under identical build conditions must produce byte-identical artifact files.

## Artifact Format Version 2

Artifact format version 2 replaces the randomized map iteration order of the function table used in version 1 with a canonical name-sorted function table.

### Compatibility Specification

Format 2 (PR #74, 2026-10-01) introduced a new HowlFrame bytecode artifact envelope. Current HowlFrame binaries still
decode format 1, but artifacts they write cannot be read by v0.1.0. Users who must run artifacts on v0.1.0 should
compile them with the v0.1.0 compiler.

| Runtime | `howlframe version` prints | Reads format 1 (`.hfbc`) | Reads format 2 (`.hfbc`) | Writes |
|---|---|---|---|---|
| v0.1.0 (published 2026-08-13) | `HowlFrame 0.1.0`, `HFBC format: 1` | Supported | **Rejected** (`unsupported artifact version: expected 1, got 2`) | Format 1 |
| Unreleased `main` builds, 2026-10-01 to v0.2.0 | `HowlFrame 0.1.0`, `HFBC format: 2` | Decoded (see below) | Supported | Format 2 |
| v0.2.0 | `HowlFrame 0.2.0`, `HFBC format: 2` | Decoded (see below) | Supported | Format 2 |

The unreleased builds kept the 0.1.0 version string after the format changed. Use the `HFBC format` line, which is
derived from the writer, to tell which artifacts a binary emits. `howlframe interop --json` reports the same pair as
`version` and `hfbc_format`.

### Backward Compatibility (Reading Older Artifacts)

A v0.2.0 runtime **decodes** format 1 artifacts: [`ReadArtifact`](../../internal/bytecode/artifact.go) reads the
legacy map-based payload and resolves opcodes by their stable names. Decoding is not the same as identical behavior.
The artifact executes under the v0.2.0 instruction semantics and capability policy, and several of those changed
after v0.1.0. Measured with artifacts compiled by the published v0.1.0 binary (`HOWLFRAME_RELEASE_COMPATIBILITY.md`
under `docs/release-readiness/v0.2.0/`):

| v0.1.0 artifact uses | On v0.2.0 |
|---|---|
| Printing, strings, lists, file reads, finite arithmetic (`cli_hello`, `log_analyzer`, `action_executor`) | Same output and exit status |
| `store_open` (`kv_cli`, `todo_cli`) | Fails closed: `STACK_UNDERFLOW` at `STORE_OPEN` (the instruction now takes its URI from the stack) |
| `spawn_agent` | Fails closed: `STACK_UNDERFLOW` at `SPAWN_AGENT` (it was already unusable on v0.1.0: `unknown opcode: SPAWN_AGENT`) |
| `map_get` of a missing key | **Different result**: `nil` instead of `""` (#137), so `release_authority` prints `"target": "unknown"` where v0.1.0 printed `""` |
| `to_float` of NaN/Inf text, float overflow, division by zero | Fails closed with `CONVERSION_ERROR` or `RUNTIME_ERROR` where v0.1.0 produced `NaN` or `+Inf` (NUMERIC_CONTRACT) |
| `confidence`, `neural_circuit`, `ephemeral_circuit` | Needs the `network` grant |
| `exec`, `fetch` | Needs the scoped grants (`process:exec=<absolute path>`, host-scoped `network:fetch=`); bare `process` and `network` no longer authorize them |
| A float constant that is NaN or infinite | Refused at load (`LOAD_CONST ... is not a finite number`) |

Recompile from source with v0.2.0 to get the current semantics deliberately. Keep a format 1 artifact only when it
must also run on v0.1.0, and verify its behavior on both runtimes, as the HowlPlane Mission007 portable artifact did
(`docs/journals/2026-10-21_mission007_frame_compatibility.md`).

### Forward Compatibility (Older Runtimes Reading Newer Artifacts)

v0.1.0 checks the version field in the artifact header:
- It expects `ArtifactVersion = 1`.
- A format 2 artifact fails closed with `ErrUnsupportedVer`:
  ```text
  unsupported artifact version: expected 1, got 2
  ```
- Newer artifacts cannot be loaded on older runtimes.

### Malformed Artifacts

Every load path (`run`, `-run-bc`, `inspect --caps`) verifies the envelope and the program structure before
executing anything, and exits with status 1 and one message:

| Input | Message |
|---|---|
| Wrong magic, random bytes, JSON | `invalid artifact magic identifier` |
| Version other than 1 or 2 | `unsupported artifact version: expected 1 or 2, got N` |
| Empty or cut short | `truncated artifact` |
| Payload or checksum altered, trailing bytes | `artifact integrity checksum mismatch or corrupted payload` |
| Declared payload over 10 MiB | `artifact payload exceeds trusted maximum size` |
| Unknown opcode, out-of-range jump, missing function, non-finite constant | `structurally invalid bytecode program: ...` |

The SHA-256 in the envelope detects accidental corruption. It is not a signature: anyone who can write an artifact can
write a valid checksum.

### Migration & Deployment Guidance

1. **Upgrading Deployments**: When updating to a runtime that reports `HFBC format: 2`, existing format 1 `.hfbc` artifacts still load, but they run under the current semantics (see the table above). Recompile them from source, or verify each one on the new runtime before relying on it.
2. **Generating New Artifacts**: When you compile `.howl` source using the updated runtime, the output artifact will be written in format version 2. If downstream systems or worker nodes still run older HowlFrame binaries, upgrade those nodes before deploying newly compiled artifacts, or compile the artifacts using a runtime matching the older target version.
3. **Artifact Hash Verification**: Because version 2 serializes functions in sorted order, artifact hashing (e.g. SHA-256) is deterministic across multiple compiler invocations and platforms.
