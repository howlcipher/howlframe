# HowlPlane Factory Preflight — HowlFrame

## Executive Summary

| Field | Value |
| --- | --- |
| Readiness classification | **READY** |
| Factory started | No (preflight only) |
| Target repository | howlcipher/howlframe |
| Target repository path | /run/media/system/tallgeese/dev/howlframe |
| Target commit (HEAD) | 508196d69dc68a027693be70f672c41a155f9819 |
| Branch | main |
| Upstream | origin/main (one commit behind local HEAD) |
| Timestamp | 2026-09-15 14:49:21 UTC |
| HowlPlane version | 0.1.0 |
| Requested authority | safe (strict) |

## Remediation Note

The previous preflight found a stale `howlframe-overnight` authority envelope that
would be reused by `howlplane factory start --authority safe`, a red
`go test ./...` baseline, and a `slopslint` production-clone ceiling violation.

The stale campaign state at
`/home/howlcipher/.local/state/howlplane/factory/e95237ce7bac3feb346602aa` has
been left intact as historical evidence. The in-flight work that caused the red
baseline has been preserved in a git stash and is no longer in the working
tree. The remaining slopslint gap in the clean baseline was closed by real
deduplication (extracting shared test helpers) and a downward ratchet of the test
clone ceiling from 138 to 135.

## Environment

| Field | Value |
| --- | --- |
| OS / distribution | Linux 7.2.4-ogc3.1.fc44.x86_64 (Fedora remix) |
| Hostname | epyon |
| Current user | howlcipher |
| Python | 3.14.4 |
| Go | go1.26.4 linux/amd64 |
| Git | 2.53.0 |
| Docker | not available |
| Disk (/) | 929G total, 169G used, 755G available (19%) |
| Memory | 23G total, 14G available |
| HowlPlane executable | /home/howlcipher/.local/bin/howlplane |
| HowlFrame repo | /run/media/system/tallgeese/dev/howlframe |

## Git State

| Field | Value |
| --- | --- |
| Branch | main |
| SHA | 508196d69dc68a027693be70f672c41a155f9819 |
| Upstream | origin/main (one commit behind) |
| Dirty/clean | **Clean** |
| Merge/rebase state | None |
| Detached HEAD | No |

The working tree is clean. The previous 16 modified tracked files and 9 untracked
files are preserved in git stash `stash@{0}` for later recovery.

## HowlPlane Health

| Check | Result |
| --- | --- |
| `howlplane --version` | 0.1.0 (exit 0) |
| `howlplane factory start --help` | `--authority {safe,standard,autonomous}`, `--state-dir`, `--target-repo` supported (exit 0) |
| `howlplane doctor` | HEALTHY (exit 0) |
| `howlplane factory doctor` | HEALTHY on old campaign worktree (exit 0) |

## Old Factory Campaign (Preserved)

| Field | Value |
| --- | --- |
| Campaign ID | e95237ce7bac3feb346602aa |
| State directory | /home/howlcipher/.local/state/howlplane/factory/e95237ce7bac3feb346602aa |
| Worktree | /home/howlcipher/.local/share/howlplane/worktrees/e95237ce7bac3feb346602aa/target |
| Authority | howlframe-overnight (expired) |
| Last tick | 2026-09-14T00:21:19.195808+00:00 |
| Last successful tick | 2026-09-14T00:21:19.193725+00:00 |
| Recent failed | 2 (WI-howlframe-90960cf278bc4ea6, WI-howlframe-2a0f112733c06562) |
| Final state | stopped (signal_sigterm) |

`campaign/envelope.json` and all other historical state files were left
untouched. This campaign will not be reused.

## New Factory Campaign Plan

| Field | Value |
| --- | --- |
| State directory | /home/howlcipher/.local/state/howlplane/factory/howlframe-safe-20260915-144921 |
| Worktree | /home/howlcipher/.local/share/howlplane/worktrees/howlframe-safe-20260915-144921/target |
| Authority | safe -> strict profile |

The strict authority profile has TTL 0h, `max_merges=0`, $0 spend, an empty
allowed-action list, and denies all consequential actions. Because a fresh
`--state-dir` with no existing `campaign/authority_envelope.json` will be used,
`--authority safe` will bind the strict profile, not the old
`howlframe-overnight` envelope.

## Baseline Verification

| Check | Command | Exit | Result |
| --- | --- | --- | --- |
| Format | `gofmt -l .` | 0 | PASS |
| Build | `go build -v ./...` | 0 | PASS |
| Vet | `go vet ./...` | 0 | PASS |
| Tests | `go test ./...` | 0 | PASS |
| SlopsLint | `slopslint check --classify --enforce` | 0 | PASS |
| HowlPlane verify | `howlplane verify` | 0 | PASS |
| CI harness tests | `python3 -m unittest test_harness.py` | 0 | PASS |
| SEO artifacts | `python3 scripts/test_seo.py` | 0 | PASS |

### SlopsLint scopes

| Scope | Active clones | Ceiling | Result |
| --- | --- | --- | --- |
| go_production | 88 | 88 | PASS |
| go_tests | 135 | 135 | PASS |

## Provider Capacity

| Provider | Installed | Ready | Role Eligibility | Notes |
| --- | --- | --- | --- | --- |
| Claude Code | yes | yes | implementation / review | — |
| Codex CLI | yes | yes | implementation / review | — |
| Gemini CLI | yes | no | — | MISSING_EXECUTABLE |
| Devin CLI | yes | yes | implementation / review | — |
| Antigravity (agy) | yes | yes | implementation | — |
| Local Ollama | no | no | — | UNREACHABLE / not on PATH |

Sufficient provider diversity exists for implementation and independent review.
Gemini and Ollama unavailability is a warning, not a blocker.

## Warnings

1. Local Ollama and Gemini CLI are unavailable.
2. Docker is not installed.
3. `howlplane doctor` reports the Python session is not inside an active
   `VIRTUAL_ENV`; all required Python dependencies are still importable.

## Blockers

None.

## Recommendation

**READY TO START FACTORY.**

Run from the HowlFrame repository directory (the currently installed CLI resolves the target repository from the current working directory for `factory start`):

```bash
cd /run/media/system/tallgeese/dev/howlframe
howlplane factory start \
  --state-dir /home/howlcipher/.local/state/howlplane/factory/howlframe-safe-20260915-144921 \
  --target-repo /home/howlcipher/.local/share/howlplane/worktrees/howlframe-safe-20260915-144921/target \
  --authority safe
```

Then verify health with:

```bash
cd /run/media/system/tallgeese/dev/howlframe
howlplane factory status --state-dir /home/howlcipher/.local/state/howlplane/factory/howlframe-safe-20260915-144921
```
