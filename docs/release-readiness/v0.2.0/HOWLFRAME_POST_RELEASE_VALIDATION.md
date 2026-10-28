# HowlFrame v0.2.0 post-release maintenance and application validation

Bounded follow-up after the publication recorded in `HOWLFRAME_RELEASE_PUBLICATION.md`: one test-harness fix, follow-up
issues, and one validation of a real consumer against the **published** v0.2.0 binary
(`af9216894e9a14148b8592fc12aacc6e1d6c6115033799a6c99882a0b223bbb7`, linux/amd64). Nothing here changes v0.2.0.

## Maintenance

### `apps/echo_api` leaked a listening server (#241, merged as `02d68f8b` on the Owner's instruction)

* **Reproduced** on `77cb55c8`: after a passing `go test -count=1 ./apps/echo_api/`, `*:8106` was held by
  `…/go-build/…/howlframe -run-bc -allow-caps network echo_api.howl.bc.bin`, reparented to the container's init.
* **Cause:** the test started the server with `go run`, which runs the compiled howlframe as a child; the deferred
  `srv.Process.Kill()` killed only the wrapper. A later run could also pass against the leftover server.
* **Fix (test file only):** process group plus group kill (the idiom four sibling app tests already use), stop through
  `t.Cleanup`, refuse to start when 8106 already answers, artifact in `t.TempDir()`. `TestEchoAPI` asserts the port
  is released after stopping; `TestEchoAPIStopsServerWhenTestFails` reruns it as a child that fails after the server
  starts and asserts the port is released on that path.
* **Evidence:** with the old stop restored, `TestEchoAPI` fails (`127.0.0.1:8106 still accepts connections after the
  server was stopped`) and so does the failure-path test. With the fix: gofmt, build, vet clean; `go test ./...` 76
  ok with Go 1.25.14 and port 8106 free afterwards; `-race` on the package ok; CI (`build`, `acceptance`, `race`,
  `latest-go`) green. Leftover processes from the reproduction were stopped one by one after matching their exact
  command line.
* **Independent review: REJECT, then one bounded correction; re-verification inconclusive.** The reviewer
  (`INDEPENDENT_REVIEW_POST_RELEASE.md`, Item 2) confirmed the original leak and the false pass against a foreign
  server, but in its sandbox `TestEchoAPIStopsServerWhenTestFails` failed: the child's server could not bind 8106
  right after the previous server was killed with keep-alive connections open. That did not reproduce here in 40
  back-to-back runs (Go 1.25.14 and 1.26.0, including a fresh network namespace) or in CI; the reviewer also ran the
  old, leaking test in the same minute, so interference cannot be excluded. Both proposed remediations were applied
  anyway (`fd71686c`): the client closes idle connections before the server is stopped, and the port counts as
  released only when it can be listened on again. The full gate passed on that state (gofmt, build, vet, 76 of 76,
  `-race` on the package, 20 more stress runs, CI green), and the old lifecycle still fails the test. The bounded
  re-verification did not finish: the reviewer's subscription quota ran out (`RESOURCE_EXHAUSTED`, resets in about
  50 hours) before it wrote a report, so the correction has not been independently re-verified.

### Follow-up issues

| Item | Tracking |
|---|---|
| Invalid resource-limit values exit 1 or 2 depending on flag and entry point (reproduced with the published binary) | #236 (new) |
| Generated Go numeric contract (checked arithmetic) | #237 (new) |
| Native macOS and Windows smoke tests | #238 (new) |
| Report an artifact's HFBC format in `inspect` | #239 (new) |
| Optional release signing and provenance | #240 (new) |
| Process containment limitations | #216, #217, #182 (existing, unchanged) |

## Real-application validation: HowlBoard

**Application.** HowlBoard (`howlcipher/howlboard`, `main` `2e17439`): a governed mission board whose backend
(`backend/server.howl`, bytecode), interface (`frontend/*.howl`, JavaScript) and ledger importer
(`tools/ledger_import`, bytecode, using EvidenceDesk's ledger module through `use`) are HowlFrame. The importer
projects HowlPlane evidence-ledger records into missions; it is the workflow HowlBoard uses for real ledger slices.

**Toolchain.** The published linux/amd64 executable placed at `howlframe_bin`, the path HowlBoard's Makefile and
tests use. No HowlFrame source checkout or development build was involved.

### Unchanged application on the published binary

| Workflow step (from HowlBoard's CI and README) | Result |
|---|---|
| `make build` (backend and importer bytecode, interface and published demo JavaScript) | Builds |
| `make test`; backend `go test -count=1 -v ./...` | Pass: 23 tests, 119 subtests, 0 fail, 0 skip |
| CI import step: 500-line slice, `MAX=500` | `entries=72`, `missions=16` |
| Full sanitized sample with `make import` budgets / with default VM budgets / with a 1 MiB memory budget | Same output each time |
| `scripts/test_docs.py`, `scripts/test_seo.py`, `scripts/test_timeline_layout.py` (Chromium) | Pass |
| Vendored ledger module | Byte-identical to `milestone3/evidencedesk/app/ledger.howl` in v0.2.0 |

Importer behavior on the published binary:

| Input or grant | Observed |
|---|---|
| `inspect --caps` | `database`, `filesystem`, with the instruction sites |
| No grants | `Failed to read ledger file`, exit 3 |
| `filesystem` only | `CAPABILITY_DENIED` at `STORE_OPEN`: `capability denied: database` |
| `filesystem:read=<repo>/data`, `filesystem:write=<repo>`, `database` | `CAPABILITY_DENIED` at `STORE_OPEN`: `capability denied: filesystem` |
| `filesystem:read=<repo>`, `filesystem:write=<repo>`, `database` | Imports (path-scoped grants suffice) |
| Malformed JSON on line 4 / duplicate `entry_id` / more lines than `MAX` | `invalid_ledger at line 4: Malformed JSON` / `at line 3: Duplicate entry_id` / `payload_too_large at line 11`; each says nothing was imported, exit 3 |

### Maintenance change (howlcipher/howlboard#24, merged as `0cb3450e` on the Owner's instruction)

HowlBoard's CI and README built HowlFrame from source at `e77e008f`, documented as "source-candidate compatibility,
not a published release assertion". v0.2.0 contains that commit. The change installs the published release through
`scripts/install_howlframe.sh`, which checks the archive against a SHA-256 pinned in HowlBoard (the release's own
`SHA256SUMS` ships beside the archive), confirms `HowlFrame 0.2.0`, and installs nothing otherwise. CI, the README
and HowlBoard's published page use it; `test_docs.py` enforces that. HowlBoard's sources, tests, Makefile and grants
are unchanged, and no HowlFrame change was needed.

Acceptance criteria were written before the change and are in the PR. Evidence: a clean copy of the branch installs
`af921689…`; a wrong pinned hash installs nothing; all HowlBoard suites above pass; the new docs check fails against
the old `ci.yml`; HowlBoard CI on GitHub (run 40756561617) installed the release with the script and passed every
step.

**Independent review: ACCEPT WITH NOTES.** Antigravity (`gemini-3.8-flash-high`, subscription login, no paid API
calls), sandboxed, from an isolated package with the five criteria and no implementer conclusions; verbatim report:
`INDEPENDENT_REVIEW_POST_RELEASE.md` (Item 1). Criteria 1, 2, 4 and 5 were met by execution, including a wrong hash,
a wrong version and an unsupported platform each installing nothing, and six docs-check mutations each failing. In the
reviewer's sandbox, five backend tests failed on `port localhost:8080 is already in use` (reported as pre-existing on
`main`, with no raw log kept for that run), and the Chromium layout test failed on `Page.add_style_tag`. Neither
reproduced locally or in HowlBoard CI; both are recorded as possible environment sensitivities of HowlBoard's
harness. No change was made in response.

### Developer-experience findings

1. **Compile-time diagnostics are actionable.** Type and parse errors give a reason with line and column, and the
   unclosed-form message says where the form opened and what to add.
2. **Runtime errors are not.** VM errors carry the function, the instruction index and the opcode, never a source
   position (`instruction 481 STORE_OPEN`); HFBC instructions hold no positions to report.
3. **Capability denials do not say what was missing.** A path outside a `filesystem:read=` root, and a store file that
   needs read and write on its own path, both report `capability denied: filesystem`, the same text as no grant.
   Finding the working grant for the importer took three runs.
4. **An application can hide the denial.** The importer's `try_let` turns a denied read into `Failed to read ledger
   file`; nothing tells the operator a grant is missing. That is HowlBoard code (it does not print the caught error),
   a candidate HowlBoard maintenance item.
5. **Budgets were not hard to tune for this input.** The sample imports under the default budgets and under a 1 MiB
   memory charge; `make import` documents why it raises the budget for 3 MiB slices.
6. **`read_file` returns bytes.** Printing it shows a list of byte values until `bytes_to_string` is added; documented
   (#230), but it caught the first smoke program in this verification.
7. **Stale consumer documentation.** HowlBoard's roadmap still lists "module support in the bytecode target" as its
   largest framework constraint, while its own importer compiles a `use` import to bytecode. Not changed here.

### What the released language helped with

| Question | Evidence from this exercise |
|---|---|
| Correctness of programs | The importer refuses malformed, duplicate and over-limit input with line numbers and writes nothing; the numeric and capability contracts held on every path exercised. |
| Actionable failures | Yes at compile time; weak at runtime (findings 2 and 3). |
| Capability-governed execution | Grants were enforced exactly, including path-scoped ones; `inspect --caps` names what a program needs before it runs. Diagnosing a missing grant is the weak point. |
| Maintainable change | The maintenance change needed no runtime change and no HowlBoard source change. |
| Reproducible installation and execution | Published executables are byte-identical to an independent build; HowlBoard can now pin the release by checksum. |
| Productivity | One maintenance PR, verified locally and in CI. No comparison with other languages was made. |

## Recommended next investment

**Runtime diagnostics that point back to the source:** VM runtime errors that carry the source line and column, and
`CAPABILITY_DENIED` messages that name the denied mode and path (and, for scoped grants, that the path is outside the
granted roots). Not started; it needs its own scope.

| Candidate | Evidence from v0.2.0 verification | Assessment |
|---|---|---|
| **Runtime diagnostics** | Every runtime failure in this exercise reported only an instruction index; three runs to find the importer's grants | Hits every runtime failure an agent or operator debugs; makes least-privilege grants practical (HowlBoard still grants coarse `filesystem`); denial detail is VM-only and small, positions need an HFBC addition designed to keep deterministic encoding and format 1/2 loading |
| Generated Go numeric checks (#237) | No consumer used generated Go for numeric decisions | Real gap, documented; no demonstrated consumer impact yet |
| Resource-budget ergonomics | HowlBoard's sample ran under default and 1 MiB budgets | Not shown to be difficult |
| Exit status consistency (#236) | Reproduced | Small, low impact |
| HFBC format in `inspect` (#239) | Needed once, read from header bytes | Small, low frequency |
| VM performance | No slow path met | No profile to act on |
| Test-server lifecycle | The one leak found is fixed in #241 | Done pending review |

Suggested order inside that scope: denial detail first (no artifact change), then a design note on carrying source
positions in HFBC, then the implementation, with contract tests on both formats.
