# HowlFrame v0.2.0 release candidate

**Status: prepared for Owner approval. Not tagged, not published.** No tag was created, no release workflow ran, no
asset or package was published, and nothing was merged.

## Verdict

**ACCEPT WITH DOCUMENTED LIMITATIONS**: the independent reviewer's verdict on the candidate, with its two findings
corrected in one bounded round and re-verified (re-verification verdict also ACCEPT WITH DOCUMENTED LIMITATIONS). Every gate passes on the final source (`29c6108c`).

## Proposed version: v0.2.0

The published v0.1.0 cannot read the artifacts current builds write (HFBC format 2), grants are narrower, and
non-finite numbers are now errors. Under Semantic Versioning's 0.x convention (the repository has no written policy)
that is a minor release; 0.1.1 would claim compatibility, and is also the name of an abandoned, never-pushed local
tag. Details: `HOWLFRAME_RELEASE_COMPATIBILITY.md`.

## Contents

| PR | Branch | Commits | Purpose |
|---|---|---|---|
| Numeric correction | `fix/numeric-nonfinite` | `2cb71aa8`, `9368676a` | Enforce NUMERIC_CONTRACT 2.3 and 7: float overflow, out-of-range literals, `confidence`, non-finite artifact constants; `%`/`//` overflow-free; generated-Go deviation documented |
| Release candidate (stacked) | `release/v0.2.0-rc` | `3cbbf35b`, `29c6108c`, and a docs-only report commit | Version 0.2.0; release workflow tag/version guard and `-trimpath`; installation guide with the real archive names; accurate format 1 compatibility; project page without sandbox claims; `SECURITY.md` known limitations; release notes `docs/releases/v0.2.0.md`; these reports |

Staged artifacts (built from `29c6108c` by replicating `release.yml`, Go 1.25.14): five archives and `SHA256SUMS`;
binary hashes in `HOWLFRAME_RELEASE_TEST_RESULTS.md`. They are reproducible: a CI build of the merged commit with Go
1.25.14 should produce the same binaries.

## Evidence summary

| Area | Result | Report |
|---|---|---|
| Numeric contract | Four defects reproduced and fixed on the interpreter, AST and HFIR bytecode and generated JS; generated Go documented as non-conforming | `HOWLFRAME_NUMERIC_CORRECTNESS.md` |
| Engineering gates (Go 1.25.14) | gofmt, build, vet, test 76/76, race 76/76 | `HOWLFRAME_RELEASE_TEST_RESULTS.md` |
| Applications with the staged binary, clean install, no Go on `PATH` | LedgerLens 35/35, HealthWatch 29/29, TaskFlow 32/32, TicketDesk 101/101 + DX, adversarial, regression, restart suites (318 PASS, 0 FAIL, 1 recorded XFAIL); EvidenceDesk with real Chromium and the actual HowlBoard consumer: all PASS | same |
| Compatibility | v0.1.0 rejects format 2 as documented; format 1 artifacts load on v0.2.0 with measured differences now documented; malformed artifacts fail safely | `HOWLFRAME_RELEASE_COMPATIBILITY.md` |
| Platforms | linux/amd64 executed; linux/arm64 under emulation only; macOS and Windows built, **never executed** | same |
| Security | Guarantees probed; #216 reproduced; nothing release-blocking in code; project-page sandbox claims removed | `HOWLFRAME_RELEASE_SECURITY.md` |

## Independent review

* **Reviewer:** Antigravity CLI, model `gemini-3.8-flash-high` (Google; the implementation was by Claude), subscription
  login, no paid API calls, run from an isolated copy of the tree and staged archives with an objective brief (no
  implementer conclusions supplied). An unrestricted-permissions run was blocked by the local safety policy; the review
  ran in Antigravity's own `--sandbox` mode and still executed programs.
* **Verdict on `bbd4c8f5`:** ACCEPT WITH DOCUMENTED LIMITATIONS. Numeric contract, binaries, acceptance, security
  claims, archives, installation and scope: PASS.
* **FINDING-01 (medium):** the docs said v0.1.0-built `spawn_agent` artifacts fail closed. Confirmed independently:
  with `(task ...)` bodies (`examples/swarm_dogfood`) they run on v0.2.0; without `task` they fail closed. Corrected in
  `artifact_compatibility.md`, the release notes and the compatibility report.
* **FINDING-02 (low):** the reports referenced by `SECURITY.md` and the release notes were not yet committed. They are
  now in `docs/release-readiness/v0.2.0/`.
* **Re-verification of `29c6108c`** (same reviewer, bounded to the correction diff): FINDING-01 **RESOLVED** (both
  `spawn_agent` forms rerun on both binaries), FINDING-02 **RESOLVED**, the correction diff confirmed documentation-only,
  and every concrete claim it added checked by execution. It raised **FINDING-03 (low)**: the numeric report cited this
  file before it was committed. Resolved by committing this file; per the one-round limit, no further review cycle was
  started. Verdict: ACCEPT WITH DOCUMENTED LIMITATIONS.
* **Reports, verbatim:** `INDEPENDENT_REVIEW_bbd4c8f5.md` and `INDEPENDENT_REVERIFY_29c6108c.md` in this directory
  (they contain the reviewer's local scratch paths).
* **Review caveats.** The reviewer reported HealthWatch 27 and TaskFlow 45 tests; the frozen suites contain 29 and 32
  (observed in four runs here) and it kept no raw log, so its acceptance item corroborates rather than independently
  proves the result. It did not run EvidenceDesk (Chromium and HowlBoard), and could not execute macOS or Windows
  binaries.

## Known limitations shipped (documented in `SECURITY.md` and the release notes)

#216 hostile supervisor kill (reproduced); #217 macOS/BSD/Windows containment; #182 `process:env=` deny-list;
generated Go numeric semantics; no exponent literals; invalid resource limits exit 1 or 2 depending on path; format 1
artifacts can behave differently (`store_open`, `spawn_agent`, `map_get` of a missing key, non-finite values, grants);
unsigned binaries; macOS and Windows binaries unexecuted.

## Decisions required from the Owner

1. **Version:** approve `v0.2.0`.
2. **Unexecuted platforms:** ship the macOS and Windows binaries labelled "built, not executed" (as the docs now say,
   and as v0.1.0 did silently), or run `howlframe version`, `check`, `build` and `run` on a hello program on a Mac and a
   Windows machine first, or withhold those archives.
3. **Limitations:** accept the limitations above for an experimental release.
4. **Public wording:** the project page correction goes live when the RC PR merges (Pages deploys from `main`).
5. **Merge order:** the numeric PR, then the RC PR (stacked on it).
6. **Follow-up issues** (not done here, out of scope): the `apps/echo_api` test leaks a listening server after every
   `go test ./...`; consistent exit status 2 for invalid resource limits; report an artifact's HFBC format in
   `inspect`; checked arithmetic in generated Go; optionally include `LICENSE` in release archives.
7. **Local housekeeping (optional):** the never-pushed local tag `v0.1.1` on `e2b62f4c` in the maintainer checkout can
   be deleted to avoid confusion.

## Approval checklist

- [ ] Review and merge the numeric PR; CI green on `main`.
- [ ] Merge the RC PR; CI green on `main`.
- [ ] On the merged commit: `go run howlframe.go version` prints `HowlFrame 0.2.0`, `HFBC format: 2`.
- [ ] Decide on macOS and Windows (decision 2).
- [ ] Tag the merged commit: `git tag -a v0.2.0 <sha> -m "HowlFrame v0.2.0"` and `git push origin v0.2.0`. The release
      workflow refuses the tag if the version disagrees, builds the five archives, requires `docs/releases/v0.2.0.md`,
      and publishes with `SHA256SUMS`.
- [ ] Download the published assets, run `sha256sum -c SHA256SUMS`, extract the Linux binary and compare its SHA-256
      with `af9216894e9a14148b8592fc12aacc6e1d6c6115033799a6c99882a0b223bbb7`. A mismatch means CI used a different Go
      1.25 patch release or different source; restage with that toolchain before trusting the match.
- [ ] Follow `docs/installation.md` once from the published URLs.
