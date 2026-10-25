# Revision request 1 for the D3/D4 pool

You authored `pool/` in this directory from BRIEF.md. The operator scored every candidate's screen implementations in
the experiment's scoring sandbox: a fresh jail per language containing only that language's toolchain plus a shared
Ubuntu base. 24 of 28 pool candidates pass. These four D3 candidates fail there, although they passed on your host:

| Candidate | Python screen | JavaScript screen |
|---|---|---|
| d3-distributed-lock-orchestrator | 11/11 | 8/11 |
| d3-parallel-test-matrix | 11/11 | 8/11 |
| d3-pipeline-runner | 12/12 | 9/12 |
| d3-sandbox-exec-policy | 7/11 | 7/11 |

**Cause:** the acceptance tests make the program launch `python3` (or `/usr/bin/python3`) as a child process. In the
experiment, Python exists only in the Python arm's sandbox, and not at `/usr/bin/python3` even there. So these tests
cannot pass in the Go, JavaScript or HowlFrame arms. The test runner's own Python is not available to the program.

**Fix:** every child process a test asks the program to launch must exist in **every** arm's sandbox, so use only
the shared base system. In that base, these are real executable files (not symlinks): `/usr/bin/dash`,
`/usr/bin/bash` and `/usr/bin/coreutils` (a multi-call binary: `/usr/bin/coreutils sleep 10`, `/usr/bin/coreutils
echo hi`). `/bin` is a symlink to `/usr/bin`, and most utilities in `/usr/bin` (`sh`, `echo`, `sleep`, `env`,
`cat`, `false`, `true`) are symlinks. Some languages' process APIs refuse symlinked programs, so name programs by
these canonical paths, for example:

* exit with code 7: `["/usr/bin/dash", "-c", "exit 7"]`
* run 10 seconds: `["/usr/bin/coreutils", "sleep", "10"]`
* print: `["/usr/bin/coreutils", "echo", "hello"]`
* a helper script: write it as a dash script and run `["/usr/bin/dash", "/path/script.sh"]`

Child processes may start with an empty environment in some languages, so a child must not rely on `PATH`; if a test
needs one, it must use absolute paths. Update the contracts (`prompt.md`) wherever they name example commands, update
the tests, and keep the screen implementations passing. Check that no other candidate (all 28, including D4) has a test
that makes the program launch `python3`, `node`, `go` or any interpreter. Change nothing else. Append a "Revision 1"
section to `pool/AUTHOR_NOTES.md` and print a short report.
