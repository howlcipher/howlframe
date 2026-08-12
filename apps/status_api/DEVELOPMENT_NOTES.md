# Status API Development Notes

## What worked well
- The `http_server`, `route`, `lambda`, and `res_json` forms are incredibly simple and expressive for writing a quick web service.
- The capability model (requiring `-allow-caps network,environment`) worked exactly as intended, producing clear errors when denied.
- Execution under the bytecode VM (`-run-bc`) supports the HTTP forms cleanly and deterministic HTTP routes work standalone.

## Awkward but workable
- Writing dictionaries required the explicit list-of-pairs format `(dict ("key" "value") ("key2" "value2"))`. We naturally want to write `(dict ("key" "value" "key2" "value2"))` but the compiler requires pairs. This friction is minor but notable.

## Missing capability
- None for this application scope.

## Compiler/runtime bugs discovered
- None.

## Capability findings
- Requires `network` to start the HTTP server.
- Requires `environment` to read `REQUIRED_ENV` and `PUBLIC_CONFIG`.
- If `environment` is denied, the route panics cleanly at runtime when requested, returning a 500 equivalent and a JSON trace log.
- If `network` is denied, the server fails to start immediately.

## Instruction-budget findings
- The normal workloads fit perfectly inside the default 100,000 instruction budget. No modifications were needed.

## Developer experience
- Was the language easy to reason about? Yes, the S-expression syntax makes the AST extremely predictable.
- What consumed the most effort? Just adjusting the dictionary syntax.
- What would a developer expect that was missing? Maybe a more idiomatic dictionary format.
- Did error messages help? Yes, the error message `{"reason":"dict expects (k v) pairs"}` immediately pinpointed the dictionary syntax error.
- Did the VM behave predictably? Yes, failed fast and closed without capabilities.
