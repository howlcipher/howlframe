# Native Telemetry Injection (#55)

**Date**: 2026-07-23

## Objective
Implement native telemetry injection (Improvement #55) to provide the foundation for agentic observability. The transpiler needs to invisibly inject `observer.Trace(...)` calls at the start and end of every function block.

## Implementation Details
1. Created a new local Go package `github.com/howlcipher/howlframe/observer` (`observer/observer.go`) with a `Trace` function. This function takes a function name and a map of variables, logs an "enter" event with variables to `telemetry.jsonl`, and returns a closure that logs an "exit" event.
2. Updated `howlframe.go` to unconditionally include `"github.com/howlcipher/howlframe/observer"` in the default `import` block of both `server.go` and `server_test.go`.
3. Handled "unused import" errors by appending `var _ = observer.Trace` to the global blank-identifier declarations.
4. Hooked code generation for function blocks:
   - **`defun`**: Added `defer observer.Trace(...)()` to log the function name and all defined arguments.
   - **`route`**: Added telemetry hook to log the `req.URL.Path`.
   - **`middleware`**: Added telemetry hook to log the middleware's wrapped route path.
   - **`spawn`**: Added telemetry hook for the background anonymous lambda.
5. Successfully ran the test suite (`go test`) and transpiled test scripts to verify generated Go code builds cleanly.

## Conclusion
The foundation for the Observer agent is now established. The next step in this theme is #56 (Standalone Observer Agent) which will parse the generated `telemetry.jsonl`.
