# HFIR model adapter resource blockers (#169, #170)

Consumer: public `DecodeCandidate` / `CompileCandidate` boundary and the VM evidence/receipt runners under `DefaultExecutionPolicy`.

## Findings and changes

The fragment cache memoized lowering but still copied expanded instruction slices at each consumer. Shared closed scalar expressions now have collision-free private functions, lowering each shared node once. Calls remain at the original evaluation sites: this shares compilation, not execution results. Variable reads, allocation, control flow and effects are not outlined. A shared effect-containing sequence still prints `1,2,1,2`, in order.

Memoized saturating expansion accounting rejects dependency depth above 2048, entry expansion above 65536 nodes, or aggregate cached-fragment expansion above 4194304 nodes. Both JSON intake and direct `Candidate` compilation enforce the gate. The depth limit preserves the existing 1500-binding let-chain fixture, required by `TestModelAdapterCoversArityTable`; the 4096-node proposal with a 4093-operation chain is rejected promptly. These limits bound lowering without widening language or authority.

Programs returned directly by `CompileCandidate` carry an ephemeral runner-owned adapter constraint. VM evidence and receipt runners apply a maximum one-second wall-clock deadline, respecting shorter caller deadlines. Sleep compares milliseconds with the remaining budget before converting to `time.Duration`, so `1e15` milliseconds cannot overflow the conversion or wait unboundedly. Adapter budget failures are `HFIR_DEADLINE_EXCEEDED` and escape `try_let`. Receipts retain the effective deadline and typed failure. HFBC serialization is unchanged: serialized artifacts retain the existing artifact-runner policy contract, rather than transporting runner authority.

## Executable evidence

* `TestAdapterSharedDepth22Bounded`: successful depth-22 JSON proposal compilation, fewer than 150 instructions, under one second and 32 MiB allocated. Observed local run: 1.89 ms and 578488 allocated bytes. A smaller shared graph executes to the expected arithmetic result.
* `TestAdapterDepthCapRejects4095Chain`: over-depth proposals, including the maximum-size chain, reject with `HFIR_TRANSPORT_TOO_DEEP` in under one second.
* `TestAdapterExpansionCap`: shared allocating expressions reject with `HFIR_TRANSPORT_TOO_LARGE`. `TestAdapterAggregateFragmentCap` separately rejects excessive summed fragment storage even when entry expansion and depth are within bounds.
* `TestAdapterDirectCandidateBounds`: direct graph construction cannot bypass intake bounds.
* `TestAdapterRunDeadlineAndSleep/{huge_sleep,busy_loop,caught_sleep,normal_sleep}`: enormous sleep returns promptly; a busy loop exceeds a five-millisecond budget and fails closed; catch cannot turn refusal into success; ordinary sleep succeeds. Every receipt reports the effective deadline.
* `TestAdapterSharedEffectsKeepOrder`: repeated effects preserve multiplicity and order.

Ten temporary mutations were tested individually and restored: disabling shared lowering, depth rejection, entry-size rejection, aggregate-fragment rejection, the direct compilation gate, default adapter deadline, sleep-budget refusal, deadline propagation through catch, receipt effective-budget reporting, and loop deadline checking. Every corresponding negative test failed; the default-deadline and sharing removals hit the three-second test timeout, bounding the mutation experiment itself.

Compatibility run: `TestHFIRBenchmarkSolutionParity` passed all 268 programs (0/268 mismatches); `TestModelAdapterCoversArityTable` passed all 386 corpus round-trips, with 126/159 forms covered and 33 fenced; `TestProdFlipCriteriaLock` and CLI resource-flag tests passed. `go build ./...` and `go vet ./...` passed. Final `go test ./...` passed, with no ignored failures.

No production HFIR flip, no change to #90 (Partial), no tip-lock refresh, and no DOM additions. Pins, freeze tests, Go/JS target fences and governed checks remain intact.
