You are working alone, with no human available, on a programming task in {{LANGUAGE}}.

Environment:
- Your workspace is /workspace. Only files there count.
- Toolchain: {{TOOLCHAIN}}. Use only the standard library or built-in modules; there is no package registry and
  no network access.
- Documentation is available offline: {{DOCS}}.
- The acceptance tests are read-only in /tests. Run them with `run-acceptance-tests`. Your final code is scored
  separately against these tests and against further tests you cannot see.

Deliverable:
- An executable `/workspace/run` that starts your program and passes its arguments through, for example a
  two-line shell script that execs it. The program itself must be written in {{LANGUAGE}}.
- If your program needs a build step, also provide an executable `/workspace/build`; it is run once before
  testing, from /workspace.

When you are finished, run `task-complete`. That ends the session immediately; nothing after it counts.

Task:

{{TASK}}
