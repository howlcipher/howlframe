# 2026-10-05: Required capabilities and governed profile (C1/C2)

## Findings and change

- `bytecode.RequiredCapabilities` scans main then sorted functions, with sites
  ordered by instruction and capability. Empty arrays encode as `[]`. Both
  `inspect --caps` and `-required-caps` read artifacts and print one compact JSON
  line without execution.
- Seven BCVM `requireCapability` sites map to registry capabilities, literal
  STORE_OPEN URI requirements, four private file-handle operations and lazy
  CALL. Store handles originate only at STORE_OPEN; aliases and child VMs do
  not introduce handles from outside the artifact. Embedded TRY_LET, SPAWN,
  SPAWN_AGENT and HTTP_ROUTE instructions remain inline.
- Lazy synthesis compiles and executes the returned source. Network alone is
  insufficient: lazy/unknown calls conservatively report all capabilities.
  Unknown STORE_OPEN URIs conservatively add filesystem even though today's
  VM rejects them before accessing storage.
- Governed uses the registry's real `spawn` and `spawn_agent` names and excludes
  both. It reuses construct-position traversal, including agent task bodies.
  Imported sources are checked before expansion under the original input
  directory, resolving existing symlinks. `use` follows the same root policy
  as `include` so it cannot bypass confinement. Go/JS output targets are rejected;
  check remains target independent and bytecode-family builds remain available.

## Verification

- Table tests cover report rules, empty encoding, ordering, repeatable JSON,
  registry capability coverage, profile exclusions, source locations and imports.
- The AST source scan locks the seven gate sites and their rule mapping.
  A model-response transport test executes a synthesized filesystem write.
- CLI tests exercise valid default forms for every exclusion, governed check
  and build rejection before output, unknown profiles, Go/JS targets and
  inspection side-effect absence. Action Executor and Release Authority pass
  governed checks unchanged.
- The tests/apps corpus compiles through the production CLI. Safe fixtures run
  with exactly their reports in a scratch directory and finite instruction/time
  budgets. External network, arbitrary process and absolute-path fixtures get
  denial-only runs. Synthetic execution covers file-store operations,
  environment, agent bodies and fetch with an in-memory HTTP transport; each
  grant's removal produces a denial. No external network is needed.

- Final formatting is clean and `go vet ./...` passes. All new C1/C2 tests
  pass. Full `go test ./...` is blocked by sandbox socket restrictions; a
  60-second per-package rerun identifies status_api/task_api cleanup hangs,
  HTTP server examples, Go/JS fetch tests, VM fetch tests and difftest's local
  listener. Existing JavaScript exec positive controls also fail with
  `spawnSync touch/printf EPERM`. No tests were weakened or skipped to hide these
  failures; full validation remains incomplete in this sandbox.

## Deferrals and limits

Legacy `-profile` support is deferred; profiles apply to check/build. Reports
are conservative program-wide unions, not reachability or path proofs. Import
checks are a static profile check, not race-proof filesystem confinement.
Production AST compilation, HFBC format and issue #90 status are unchanged.
