# HowlFrame Repo Analyst

HowlFrame Repo Analyst is a deterministic, multi-module reference application written in HowlFrame. It recursively discovers a repository's regular files, classifies common source and configuration files, reads likely text files, counts `TODO` and `FIXME` markers, identifies tests and likely entry points, and emits a stable structured report.

If multiple text files tie for the largest byte count, `largest_text_file` is empty while `largest_text_file_bytes` retains the shared maximum. This keeps the report independent of filesystem enumeration order.

The application deliberately uses HowlFrame's current standalone path. It compiles to HowlFrame bytecode and runs in the HowlFrame VM; it does not generate Go or JavaScript application code.

## Build and run

From the repository root:

```bash
go build -o /tmp/howlframe-repo-analyst howlframe.go
/tmp/howlframe-repo-analyst -validate examples/repo_analyst/repo_analyst.howl
/tmp/howlframe-repo-analyst -compile-bc examples/repo_analyst/repo_analyst.howl -o /tmp/repo_analyst.hfbc
/tmp/howlframe-repo-analyst -run-bc -allow-caps process,filesystem /tmp/repo_analyst.hfbc <repository-path>
```

The default runner policy permits 100,000 instructions. For a larger legitimate workload, the trusted runner can authorize a higher finite ceiling without changing the application or artifact:

```bash
/tmp/howlframe-repo-analyst -run-bc -allow-caps process,filesystem --max-instructions 1000000 /tmp/repo_analyst.hfbc <repository-path>
```

Pass a second application argument to write the report instead of printing it:

```bash
/tmp/howlframe-repo-analyst -run-bc -allow-caps process,filesystem /tmp/repo_analyst.hfbc <repository-path> /tmp/repo-report.txt
```

The runtime policy grants exactly the effects used by the application:

- `process` runs the host `find` command for recursive discovery.
- `filesystem` reads candidate text files and, when requested, writes the report.

No network, environment, database, model, or agent capability is used. Host `find` is an intentional composition of today's primitives, not a Repo Analyst-specific runtime operation.

## Execution policy boundary

Omitting `--max-instructions` preserves the 100,000-instruction safe default. Repo Analyst's own directory intentionally exceeds that ceiling and returns structured `LIMIT_EXCEEDED`; the same bytecode artifact completes when the runner supplies the explicit 1,000,000-instruction policy above. The value must be a positive finite integer. Zero, negative, malformed, and overflowed values fail before execution, and no unlimited sentinel exists.

This option belongs to the trusted runner, not the HowlFrame program. Repo Analyst cannot modify it through source, HFIR, bytecode, or application arguments. A larger instruction budget also grants no external capability: `process` and `filesystem` remain separately required and independently enforced.

## Modules

- `repo_analyst.howl` owns CLI handling and aggregation.
- `discovery.howl` performs recursive discovery.
- `classification.howl` contains reusable file predicates.
- `text_analysis.howl` counts bytes and markers in ordinary HowlFrame.
- `report.howl` renders the versioned `howlframe.repo_analyst/v1` format.

## Exit codes

- `0`: analysis or report writing completed.
- `2`: the repository argument was omitted.
- `3`: recursive repository discovery failed.
- `1`: the compiler or VM rejected the program, artifact, capability policy, or another unhandled runtime operation.

The Go integration test builds the HowlFrame tool into a temporary directory, copies and compiles all five modules only to HowlFrame bytecode, deletes the copied `.howl` sources, and then runs the bytecode VM against temporary repository fixtures. It verifies the exact report, repeated-run determinism, deterministic largest-file tie handling, optional file output, application exit codes, default `LIMIT_EXCEEDED`, larger-budget success with the same artifact, and capability denial even under that larger budget.
