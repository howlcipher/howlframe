# Store lock release on failure (#190), 2026-10-19

**Problem.** In `internal/vm/vm.go`, `STORE_PUT`/`STORE_DELETE` took `store.mu` and released it only on the
success path. A panic in between leaked the lock, and every later store request in the process blocked forever.
`STORE_PERSISTENCE_WRITE_FAILED` from `persistStore` is enough to trigger it. `STORE_GET`/`STORE_KEYS` had the
same pattern with `RLock`.

**Fix.** All four store opcodes now go through `withStoreWrite` / `withStoreRead`, which release the lock via
`defer`. Any panic that isn't a structured `*VMError` (for example a record-copy failure) becomes
`STORE_OPERATION_FAILED`. `*VMError`s propagate unchanged, so an HTTP handler still answers 500 through its
existing recover. A test-only hook (`storeCriticalSectionHook`, an atomic pointer that is nil in production)
lets tests inject a panic mid-operation.

**Evidence.**
* `internal/vm/store_lock_test.go`:
  * A real persistence failure in a put or delete (missing directory), then put/get/keys/delete each complete within 2 s.
  * An injected panic in put/delete/get/keys becomes `STORE_OPERATION_FAILED`, and the same follow-ups succeed.
* Mutation checks:
  * With the original `vm.go`, the persistence test fails on the 2 s deadline ("store lock leaked").
  * With the helpers but no `defer`, all 6 subtests fail.
* `gofmt -l .` clean, `go vet ./...` ok, `go test -count=1 ./...` 76 packages ok. The ticketdesk regression suite
  passes 9/9 (main) and 10/10 (#189's suite, including the run-24 test). `clean_repro.sh` was run on the committed tree.

**Why it matters for M2.** This is the candidate root cause of the Round 4 run-24 cleanup-DELETE hang (one store
failure leaving every later store request hung). It isn't proven, because Round 4 captured no dump. Round 6 (pre-registered
in #189) should run on a main that contains this fix.
