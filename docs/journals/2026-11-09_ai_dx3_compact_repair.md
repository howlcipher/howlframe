# AI-DX3 compact graph repair session

Status: initial internal graph transport slice implemented and locally verified, 2026-11-10; integration pending public CI and hygiene decision. Base revision `51edd94ea349f05a289e80190f51023e35aab5b5`.

## Scope and architecture

The initial slice retains trusted graph repair authority in an opaque process-local session and exports a compact model request. The response identifies that context and proposes existing same-kind node replacements. The session reconstructs the protected-node preconditions locally and delegates to the existing atomic repair and verification gate. The digest identifies the context; it is not a signature or standalone authorization credential.

Reusing the existing transaction avoids a second repair policy and removes the repeated protected-node map from transport. Keeping authority in memory avoids persistent key management but cannot resume after process interruption. A signed serialized context could support restoration, at the cost of separate key ownership, replay and storage policies. That design is deferred.

This is graph transport, not `.howl` source editing. Existing line/column provenance does not establish lossless source spans, whitespace/comment preservation or source round trips. No source rewrite, dependency insertion, new CLI or production AST compilation change is included. The experimental graph path retains its verifier gate. Behavioral correctness still requires independent task checks.

## Test impact assessment

Observable change: trusted graph repair contexts can be exported and their proposals applied through a compact bounded interface without transmitting protected-node maps. Existing model-adapter, semantic-repair adversarial, localization and negative-state provenance tests remain intended and unchanged. New coverage must prove deterministic transport, stale and unauthorized rejection, context alias isolation, unknown/malformed/bounded transport rejection, protected region preservation, atomic rejection and successful compile/execute behavior. No tests are removed or relaxed. Source round trips are unclaimed because this interface never edits source.

Use focused compact-session tests, relevant HFIR tests, then uncached full Go tests, vet and build. Independently review authority and transport behavior. Require all four public PR and merged-main checks before completion. Record final-source hashes and actual resource events. The inherited hygiene failure remains a failed gate; the PR #307 exception does not authorize this change.

## Measurement

Freeze the protocol before implementation. Compare legacy context plus delta against compact request plus reply, excluding common task/diagnostic/provider scaffolding from both sides on the unchanged localization corpus. Include unavailable repairs and local retained-state costs. Repeated constructor, request-copy and full-gate allocation/timing measurements use an explicit three-node fixture only. Corpus-wide repeated timing/allocation comparisons remain pending; the full measurement protocol is not claimed complete. Payload byte counts establish transport costs only, not model-token savings or end-to-end efficiency. Live-agent comparisons remain deferred under the no-payment constraint.

## Verification

Final scoped focused tests and the full HFIR subsystem pass; the surrogate red regression failed in four authorized lossy-replacement cases before correction, then passed with valid pair/backslash/U+FFFD controls. Independent review and rereview close DX3-R1 (medium surrogate normalization) and DX3-R2 (low nested metadata alias/determinism coverage). DX3-R3 remains an explicitly documented measurement follow-up, with no efficiency claim.

Final local vet, build and uncached full Go tests pass: 17.351s, 15.704s and 994.773s. Formatting is clean; `go mod tidy` left dependency files unchanged. Go 1.26.0, Node 22.22.1, wasmtime 47.0.3 and wasm-tools 1.254.0 were available. The final 1,477-input source/test/configuration hash inventory matched after regression. SEO and newly added local links pass.

The final full run used fresh bounded Go-only scopes: MemoryHigh 1 GiB, MemoryMax 2 GiB, swap 0, CPU 100%, tasks 128; GOMAXPROCS 2, package parallelism 1, GOMEMLIMIT 384 MiB; launch 8 GiB and runtime 7 GiB host reserve. Kernel memory peak 656,089,088 B, tasks peak 63, minimum host available 8,025,423,872 B; scope swap 0 and resource events unchanged at zero. Maximum host PSI some/full avg10 was 0.55/0.55. Host swap counters changed and are retained without attribution to this scope. One earlier HFIR launch was refused for transient headroom; successful retry and original refusal remain separate. Initial tests-first diagnostic executions were not scope-bounded and are not final resource evidence. No browser engine or paid model call was launched.

The control-plane overall result remains **failed** solely for inherited hygiene bug #72: production active clones 745 versus ceiling 88, unchanged from main; production duplicated lines 12,826 unchanged. Test active clones 644 unchanged; test duplicated lines 10,372 versus baseline 10,366, disclosed rather than called identical. No ceiling, exclusion or test was weakened. PR #307's owner exception does not apply to this change; integration requires a separate explicit disposition and all four public checks.

## Deterministic measurement results

| Nodes | Full graph proxy | Legacy context + delta | Compact request + reply | Outcome |
| --- | ---: | ---: | ---: | --- |
| 26 | 3,743B | 5,162B | 1,413B | Recompiled and behavioral output passed |
| 53 | 7,972B | Unavailable | Unavailable | Ambiguous localization grants no edit authority; session rejected |
| 109 | 16,486B | 19,576B | 1,413B | Recompiled and behavioral output passed |

Common task, diagnostic and provider scaffolding is excluded from both boundaries. Protected authority remains local (2,478B and 9,685 B serialized context for successful scenarios); this is not total live heap cost. The three-node repeated microbenchmark reports constructor 88,977–95,871ns/op at 124 allocs/op; request copy 166.8–272.9ns/op at 1 allocation; apply/full gate 332,306–740,416ns/op at 1,051–1,052allocations. These short observations are baseline costs, not matched performance wins. Corpus-wide repeated legacy/compact timing and allocation comparisons remain pending.

Evidence: `/run/media/howlcipher/tallgeese/dev/worktrees/ai-dx3-evidence`, including preregistered protocol, original failures, scoped resource events, final-source hashes, independent reports, reconciliation, TIA and full verification. Public PR/main outcomes will bind integration evidence to the exact signed revision.


Evidence index SHA-256: `e5f4b73164ed45486c107b0720093b1486d67b17ac97e58308fb5004f7e1a5d4`.
