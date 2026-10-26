# Brief: write the held-out tests for the competitive-v1 calibration task

An experiment compares how well AI coding agents complete one programming task in four languages. Another author
(Antigravity, Google) wrote the task contract (`task/prompt.md`) and the visible acceptance tests
(`task/acceptance/`). The agent sees both. Your job is to write the **held-out tests**: a second, sealed suite the agent
never sees, used to score whether the solution really meets the contract. The protocol requires them to come from a
different author and provider than the acceptance tests, which is why you are writing them.

This task is used once, for an infrastructure check, with Claude Code as the agent; it is not a pool task.

Work only inside this directory. Do not read any other directory, do not use the network, and do not install anything.

## What to write

Create `heldout/test_*.py`: standard-library `unittest` modules that import `testkit` (see `kit/testkit.py`; use
`testkit.run([...])` for command-line programs or `testkit.Server(...)` for HTTP services). They run with Python 3.12 from
a read-only directory, so use `tempfile` for scratch files.

* Test **only behaviour that `task/prompt.md` states**. If the contract is silent or ambiguous on a point, do not test
  it; list it under "Ambiguities" in your notes instead. A correct implementation of the contract must pass every test.
* Cover what the acceptance tests do not: edge and boundary values, failure and error paths, malformed input,
  ordering and determinism, large-but-reasonable inputs, and (if the contract involves a server or concurrency)
  concurrent requests. If the contract involves access rules, test them as stated.
* Do not duplicate the acceptance tests. Aim for 15 to 30 focused tests, each with a descriptive name.
* Keep each test fast (well under 10 seconds) and deterministic; no sleeps longer than needed, no reliance on timing
  except where the contract states a deadline.

Also write `HELDOUT_NOTES.md`: what each test file covers, the ambiguities you found in the contract, and a one-line
attestation that you wrote these tests new for this purpose without seeing any implementation of the task.

You have no implementation to run the tests against. Make sure every file at least imports and is collected:

```sh
CV1_APP=/bin/false python3 -c "import sys, unittest; sys.path[:0]=['kit','heldout']; \
  print(unittest.defaultTestLoader.discover('heldout', top_level_dir='heldout').countTestCases())"
```

When you finish, print the number of tests, a one-line summary of each file, and the ambiguities.
