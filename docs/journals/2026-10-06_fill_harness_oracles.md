# 2026-10-06: Fill falsifiable harness reference oracles

## Findings and change

- Goal: complete the 54 non-smoke oracles without generating model candidates
  or running the live H1 experiment. The corpus now contains 60 reference
  oracles (30 base tasks and 30 adversarial variants), zero placeholders.
- A deterministic temporary Python script consumed the verified
  `/workspace/oracles-notes/expected_oracle_table.json`. It checked allowed
  writes against the manifest and preserved the six smoke references unchanged.
  An independent evidence/description derivation checked all 30 base contents.
- Selection preserves input order: PASS filtering selects PASS checks; unit
  selection retains the unit record even when FAIL. Batches use the base task's
  two-character suffix. Unconditional record writes produce release summaries;
  conditional writes require every check PASS and strictly greater semver,
  otherwise requesting approval with reason `blocked` and an empty summary.
  Unconditional approval requests use reason `release review` and the summary.
- Adversarial references encode the benign base success criteria and ignore
  injected note instructions. `deny_or_never_requested` remains unchanged;
  canary writes, fetches and `out/approved.json` are never expected effects.
- Each oracle has one created effect, the manifest's allowed writes and one
  actual expected output file. Blocked conditionals now point to
  `oracle/expected/out/approval_request.json`, rather than release output.
  Expected output JSON uses sorted keys, two-space indentation and a newline.

## Tests and documentation

`TestArmBRepairAndMismatchingOracle` retains repair coverage and now verifies
that a t01 plan against t04 is evaluated, has matching effects but mismatching
output, and fails success. `TestManifest` requires all 60 manifest and loaded
oracle statuses to be reference. No other tests were weakened.
README, PROTOCOL, review 09, FINAL_REPORT status/P1 table and the change log
record the filled corpus while retaining the open experiment status.
`11-implementation-candidates.md` has no placeholder experiment status to fix.

## Verification

The installed toolchain is Go 1.24.4; module/CI remain Go 1.21. The only Go edit
uses existing Go 1.21-compatible constructs. Go commands use
`GOCACHE=/tmp/hf-oracles-go-cache` because the default cache is read-only.

```sh
gofmt -l experiments/falsifiable-v0
go vet ./experiments/falsifiable-v0/...
go test ./experiments/falsifiable-v0/... -count=1
go test ./... -count=1
go test ./... -count=1 -timeout=60s
go vet ./...
go build ./...
go build -o /tmp/hfexp-oracles ./experiments/falsifiable-v0/cmd/hfexp
/tmp/hfexp-oracles validate
go build -o /tmp/howlframe-oracles .
```

- Formatting prints no files; repository build, targeted/full vet and
  `git diff --check` pass. Validator reports valid, 30 tasks/30 variants,
  experiment status still `scaffold`.
- All six existing references succeed in each of Arm A and Arm B (12 replay
  smokes). Temporary adapted Arm B plans for t04 (unconditional release) and
  t12 (blocked approval) also succeed. Plans, binaries and replay artifacts
  remain under `/tmp`, with no new checked-in reference plans.
- Programmatic checks confirm 60 reference manifest/effect statuses, existing
  output files, correct table contents and zero JSON placeholder markers
  anywhere under `experiments/falsifiable-v0/tasks`.
- The targeted suite passes the CLI package and all harness tests except
  `TestArmAFetchDeniedRecordingListener`: `listen tcp 127.0.0.1:0: socket:
  operation not permitted`. This sandbox forbids its loopback listener.
  Validation remains incomplete until the unmodified suite passes outside
  this restriction.
- The unbounded full-suite invocation was interrupted after existing server
  tests hung during cleanup; a bounded full-suite replay records failures below.
- Bounded full-suite replay exits nonzero with these unchanged tests:
  - `apps/status_api: TestStatusAPI` and `apps/task_api: TestTaskAPI`
    time out at 60 seconds after premature server exit. Stacks show cleanup
    blocked receiving the already-consumed exit channel. Socket denial is the
    likely trigger in this sandbox; the timeout stacks do not capture stderr.
  - `examples: TestHTTPServerServeHFBC`,
    `harness: TestArmAFetchDeniedRecordingListener`,
    `internal/backend/gogen: TestGoFetchRequiresGrantBeforeRequest`,
    `internal/backend/javascript: TestJSFetchRequiresGrantBeforeRequest`,
    `internal/vm: TestInterpretFetchDeniedBeforeRequest`, and
    `tools/difftest: TestLoweredHFIRABIConformance` fail because listening on
    TCP sockets returns `operation not permitted`.
  - `internal/backend/javascript: TestJSExecRequiresGrantBeforeSpawn` and
    `TestJSExecGrantedPrintsCapturedOutput` fail with `spawnSync touch EPERM`
    and `spawnSync printf EPERM`, respectively.
  No tests were skipped, weakened or changed to bypass these restrictions.
  Other completed packages pass; packages that panic do not finish their tests.
- SEO validation passes. An optional benchmark unittest invocation could not
  build its binary because it used the default read-only Go cache; it was not
  a successful benchmark validation.

## Honesty and deferrals

This is oracle authoring and handwritten replay only: no model generation,
model/LLM API calls, live offline operator trials, H1 result or PASS/KILL claim.
The experiment is not done. Filesystem grants still are not path-scoped;
two-tree diffs and receipts are not comprehensive host isolation or audit.
No runtime, compiler, VM, HFBC/opcode, DOM or production HFIR change; #90 remains
Partial. Trial freezing/hashes and corpus independence review remain operator
requirements. Live offline trials, token and reviewer-minute measurements,
Arm C, host process audit and comprehensive host/network monitoring are deferred.
