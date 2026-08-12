# Log Analyzer

A useful deterministic command-line log analyzer built in HowlFrame.

## Usage

```bash
go run ../../howlframe.go -compile-bc log_analyzer.howl
go run ../../howlframe.go -run-bc -allow-caps filesystem log_analyzer.howl.bc.bin <log-file>
```

It inspects a text log and produces a summary like this:

```text
HowlFrame Log Analyzer
file=application.log
lines=120
errors=4
warnings=9
todo=0
status=ATTENTION
```

## Exit Semantics
* `0`: Analyzed successfully, no serious errors.
* `1`: Analyzed successfully but errors/fatal entries found.
* `2`: Usage/input error (e.g. missing file).
