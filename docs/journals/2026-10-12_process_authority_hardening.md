# Process authority hardening: exec allow-lists and spawn_agent attenuation

Date: 2026-10-12. Base: main `54009fb2`.

## Why
Seat vote (Lain): before any live-model run, `process` allowed any command and
`spawn_agent` children inherited the parent's full grants.

## What changed
* `process:exec=<abs path>[ <pattern>...]` exact-argv allow-list; a bare
  `process` grant fails closed for `exec` on the interpreter, `-run-bc` (AST and
  HFIR artifacts) and generated Go/JS. No shell, no PATH lookup, symlinked or
  non-clean program paths refused.
* `spawn_agent` children get only `(grants ...)`, each covered by the parent;
  default none. Spawning still needs coarse `process`.
* Denials: `CAPABILITY_DENIED` with reason; `--receipt` records denied
  `EXEC`/`SPAWN_AGENT` process effects.

## Evidence
* `authority_cli_test.go`: allow-list, argv injection (shell metachar, extra,
  short, args smuggled into the program string), bare/relative/dotdot/symlink
  program paths, default-no-grant children, child over-request, nested
  escalation, exec delegation — on interpreter, `-run-bc` and HFIR. Interpreter
  and HFIR refuse `spawn_agent` (fail closed) and the test proves no child body
  runs there.
* ABI conformance gained `exec_bare_process_denied` and
  `exec_argv_mismatch_denied` across all five targets.
* Unit tests: `internal/capability/process_test.go`.

## Migration found while updating consumers
* `examples/repo_analyst` (find), the parity/conformance exec fixtures
  (printf, rm) and the swarm dogfoods (nested spawner needs
  `(grants "process")`) were updated. `/bin/sh` and `/bin/echo` are symlinks on
  usr-merged hosts and must be called by canonical path.

## Not changed
No opcode; #90 stays Partial; no production flip; no live LLM.
