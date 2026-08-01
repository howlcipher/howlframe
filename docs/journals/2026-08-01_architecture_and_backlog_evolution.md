# Architecture and Backlog Evolution

Date: 2026-08-01

## Scope

Architectural audit and backlog evolution only. No compiler, runtime, backend, fixture, or generated-code behavior was intentionally changed.

## Evidence Reviewed

- Root documentation, design notes, benchmark, bytecode reference, journals, bug backlog, and improvement backlog.
- Compiler entry points (`zero.go` and `cmd/zero/main.go`), lexer, parser, AST, checker, shared IR, SSA, bytecode, VM, Go/JS/Wasm backends, masking, optimization, observer, orchestrator, tests, and examples.
- Live verification: `go test ./...` passed; `python -m pytest -q tests/test_observer_agent.py` passed.

## Confirmed Findings

1. The project has several real execution and backend paths, but no canonical semantic representation. AST, shared IR, SSA, bytecode, and backend walkers overlap.
2. The documented `zero.go` CLI runs `checker.Check`; an ignored local `cmd/zero/main.go` scaffold has a separate older path that does not. It is not part of the tracked product surface and was preserved rather than altered.
3. Open bug #39 reproduces. Running `go run zero.go examples/cli_hello.zero -o /tmp/zero-architecture-output` ignored the post-input output flag and generated `server.go` in the repository root. The generated artifact was removed immediately after confirmation.
4. Bytecode capability metadata exists but the VM does not enforce a policy gate.
5. Current mask plans are provider-neutral constraints, but the local orchestration experiment does not produce a provider-neutral verified semantic artifact.

## Backlog Decisions

- Added #85 through #92 to `improvements.md`.
- Prioritized compiler ownership/output correctness, canonical ZIR, verifier/diagnostics, and model-adapter contracts before further backend feature expansion.
- Retained #73, #78–#80, #82, and #84 as useful work, but sequenced Wasm-surface expansion behind the ZIR/lowered-backend contract.
- Deferred #74, #81, and #83 relative to the new foundation because they add surface area without closing the semantic or verification gap.

## Deliverables

- `docs/architecture_roadmap.md`: evidence-based target architecture, risks, improvement and bug decisions, milestones, technology tradeoffs, and measurable success criteria.
- `improvements.md`: eight new ranked, dependency-aware foundational work items.
- `README.md`: link to the architecture roadmap.
