# Fresh taskflow candidate, 2026-10-20

Written independently from the frozen requirements after the owner's clarification to
start again. Historical source was first checked as a baseline, and a dry-run environment
bug was corrected there; the new candidate is separate fresh implementation work.

The lexer stores every command word as typed literal/reference segments. Single-quoted
text stays literal at load time, and expansion operates only on reference segments, never
on inserted text. Task records hold declarations, dependencies, descriptions and parsed
commands. DAG postorder uses a dictionary of visit states; a declaration pre-scan permits
an earlier missing dependency to win over a later parse error. Earlier cycles also win over
later malformed lines. Expansion validates every planned command before execution.

Architecture comparison: segmented words make quoting and argument boundaries explicit
and prevent accidental rescanning. They consume more intermediate records than a single
string scanner. Recursive graph and variable traversal are clear and fit the frozen scale
checks, but depend on runner call-depth/instruction ceilings. CLI overrides remain literal
because the frozen argv suite explicitly expects dollar signs in override values to remain
literal. The written recursion rule can be read differently; no contract clarification is
invented. Multi-file report/history preflight still lacks transaction/rollback support:
history is validated and preflighted first, so report refusal may follow history preflight.
No runtime changes or frozen input/test changes are included.

Working-tree compiler: `/tmp/m1-how`, built with `go build -o /tmp/m1-how .` on host Linux,
Go 1.26.0 linux/amd64 and Python 3.14.4. Candidate build and frozen suite invocation:

```
/tmp/m1-how build milestone1/rebuild/03-taskflow/src/taskflow.howl -o /tmp/rebuild-taskflow.hfbc
HF_BIN=/tmp/m1-how APP_ARTIFACT=/tmp/rebuild-taskflow.hfbc python3 milestone1/03-taskflow/tests/acceptance/run_acceptance.py milestone1/03-taskflow/tests/acceptance/launcher.example.sh -v
```

Initial candidate testing exposed list-header copying: a list appended through a function
parameter did not update the caller's length. Planning now stores its list in a dictionary
and explicitly replaces that value after appending; command records similarly retain the
updated command list. The next full run passed 31/32, identifying missing-dependency error
precedence; the declaration pre-scan fixed it, and the targeted frozen test passed.

Additional durable probes in `milestone1/tests/regression/fresh_boundaries.py` verify nested
set-variable environment references stay verbatim during dry-run, while normal run denies
without the environment grant; arbitrarily large positive `--last`; early cycle precedence;
and variable-cycle rotation to the earliest definition. All passed after cycle-rotation
repair. The historical source reproduced the nested dry-run denial (77).

The implementer is not the independent final reviewer. No cross-provider identity or OS
sandbox isolation is claimed. Historical reviews remain preserved in their original paths.

Final full frozen acceptance after cycle-rotation repair: **32 tests, 32 passed, 0 failed**.
Durable transcript: `evidence/2026-10-20_fresh_acceptance.txt`. One subsequent parser extension
accepts tab after the `task` keyword consistently with the other whitespace separators;
the four frozen T-R2 tests and a separate tabbed-directive probe passed after that change.
Added boundary probes passed against the fresh ledgerlens and taskflow artifacts (healthwatch
still the baseline artifact at this implementer's test point; its fresh candidate is separate).

## Independent review repair, 2026-10-21

A denied report path could leave a newly created writable history file. Root
added observational scoped filesystem queries to granted and checks both write
grants before creating either output. Canonical containment and symlink denial
are unit tested. The reviewer verified no partial file on denial and reran 32/32.
