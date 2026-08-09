# Zero Repo Analyst

Zero Repo Analyst is a deterministic, multi-module reference application written in Zero. It recursively discovers a repository's regular files, classifies common source and configuration files, reads likely text files, counts `TODO` and `FIXME` markers, identifies tests and likely entry points, and emits a stable structured report.

If multiple text files tie for the largest byte count, `largest_text_file` is empty while `largest_text_file_bytes` retains the shared maximum. This keeps the report independent of filesystem enumeration order.

The application deliberately uses Zero's current standalone path. It compiles to Zero bytecode and runs in the Zero VM; it does not generate Go or JavaScript application code.

## Build and run

From the repository root:

```bash
go build -o /tmp/zero-repo-analyst zero.go
/tmp/zero-repo-analyst -validate examples/repo_analyst/repo_analyst.zero
/tmp/zero-repo-analyst -compile-bc examples/repo_analyst/repo_analyst.zero -o /tmp/repo_analyst.zbc
/tmp/zero-repo-analyst -run-bc -allow-caps process,filesystem /tmp/repo_analyst.zbc <repository-path>
```

Pass a second application argument to write the report instead of printing it:

```bash
/tmp/zero-repo-analyst -run-bc -allow-caps process,filesystem /tmp/repo_analyst.zbc <repository-path> /tmp/repo-report.txt
```

The runtime policy grants exactly the effects used by the application:

- `process` runs the host `find` command for recursive discovery.
- `filesystem` reads candidate text files and, when requested, writes the report.

No network, environment, database, model, or agent capability is used. Host `find` is an intentional composition of today's primitives, not a Repo Analyst-specific runtime operation.

## Current boundary

The VM currently fixes every `-run-bc` execution to 100,000 instructions. The checked-in bounded fixture passes, but Repo Analyst exhausts that budget when pointed at its own 20,149-byte directory. The dogfooding journal records a runner-controlled instruction budget as the next blocker; the application cannot and should not raise its own safety policy. This limitation is intentionally left unresolved in the session that discovered it.

## Modules

- `repo_analyst.zero` owns CLI handling and aggregation.
- `discovery.zero` performs recursive discovery.
- `classification.zero` contains reusable file predicates.
- `text_analysis.zero` counts bytes and markers in ordinary Zero.
- `report.zero` renders the versioned `zero_repo_analyst/v1` format.

## Exit codes

- `0`: analysis or report writing completed.
- `2`: the repository argument was omitted.
- `3`: recursive repository discovery failed.
- `1`: the compiler or VM rejected the program, artifact, capability policy, or another unhandled runtime operation.

The Go integration test builds the Zero tool into a temporary directory, copies and compiles all five modules only to Zero bytecode, deletes the copied `.zero` sources, and then runs the bytecode VM against a temporary repository fixture. It verifies the exact report, repeated-run determinism, deterministic largest-file tie handling, optional file output, capability denial, and application exit codes.
