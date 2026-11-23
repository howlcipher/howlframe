# 2026-11-23: spawn and input resource bounds (bugs.md #73 to #76)

## Consumer evidence

HowlPlane's TUI-00 feasibility gate (howlcipher/howlplane
`documentation/journals/2026-11-22_tui00_feasibility_gate.md`) probed the
bytecode VM for an interactive terminal and found four places where a run
could exceed the bounds the runner set. Each has a minimal reproducer in
howlcipher/howlplane `integrations/howlframe/tui00/reproducers/` and a strict
`xfail` test there that asserts the correct behavior. All four reproduced on
`9c739c9` and on the HowlPlane CI pin `41d56ce`.

| Bug | Observed before the fix |
| --- | --- |
| #73 | five `spawn` bodies finished 500 loop iterations under `--max-instructions 1500`; the same work done sequentially fails |
| #74 | a spawn body and its parent writing one captured dict ended the process with Go `fatal error: concurrent map writes` in 5 of 5 runs, with no structured error and no receipt |
| #75 | `read_line` accepted a 32 MiB line under `--max-memory-bytes 1048576` |
| #76 | 4.5 MB of small JSON objects decoded under an 8 MiB budget while the process reached about 225 MiB RSS |

## Change

* `spawn` shares one atomic instruction counter with its spawner for the
  whole spawn tree, inherits `spawnDepth + 1`, `callDepth` and `startup`, and
  refuses nesting beyond `--max-call-depth`, as `spawn_agent` already did.
  HTTP request handlers keep their own per-request budget.
* A spawn body works on its own copy of every captured dict (and of lists that
  may hold dicts), made in the spawner's goroutine and charged to the memory
  budget. Lists are already copy-on-append. Behavior change: a body's dict
  writes no longer reach the spawner; without a join they could only ever be
  observed racily. Documented in `docs/reference/process_authority.md`.
* `READ_LINE` reads in `bufio`-sized chunks and charges each chunk before
  keeping it.
* `PARSE_JSON` charges the decoded value with the VM's own slot sizes (8 bytes
  per list item, 16 per dict entry, plus string and key bytes). Decoding still
  allocates first, so a transient peak above the budget is possible; the run
  fails closed at `PARSE_JSON`.

## Verification

* New tests: `internal/vm/spawn_isolation_test.go`,
  `internal/vm/input_memory_charge_test.go`. Each new failure test fails
  without the change and passes with it; the D2 test reproduced the Go fatal
  error before the fix.
* `go test -race -count=3` on the spawn, spawn_agent, memory and input tests is
  clean. A shared `bytes.Buffer` written by parent and child `print` is a test
  artifact, so those tests use the existing `safeBuffer`.
* `go vet ./...`, `go build ./...` and `go test -p 1 ./...` pass, except
  `TestFetchSecurity/MulticastDial` and `TestGoFetchSecurity`, which fail
  identically on unchanged `9c739c9` in a sandbox without IPv6
  (`dial tcp [ff02::1]:80: socket: address family not supported by protocol`).
* End to end: HowlPlane's TUI-00 suite run against this build turns all four
  strict `xfail` proposals into `XPASS(strict)` while its other 25 checks pass.
