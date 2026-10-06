# Offline operator runbook

The source of truth is [review 09](../../docs/ai-native-language-review/09-first-falsifiable-experiment.md).
This scaffold performs no model runs. Keep candidates, prompts, generated
artifacts and results outside the repository. Never run hostile candidates on a
shared host: use a throwaway container/VM with only the frozen experiment data,
trusted binaries, and empty writable trial space. The filesystem grant is not
path-scoped. Keep model credentials and the model-generation environment outside
that container. No model adapter belongs in CI or this harness.

1. Review the 30 tasks and one-to-one 30 variants without consulting candidates.
   Confirm each needs 3–6 logical steps, about one third loops and conditionals,
   one bounded mutation, identical grants and write permissions in each pair.
   Review the synthetic starter corpus for sufficient diversity/difficulty and
   genuine held-out status. Preserve the documented 09 thresholds.
2. Verify the 60 filled reference oracles before generation. Exact expected
   outputs and effects are authored in `tasks/<id>/oracle/`; all manifest and
   expected-effects statuses are `reference`. Preserve these frozen values.
   Effects use `{kind:"created"|"modified"|"deleted",path:"out/..."}`. The
   empty initial output directory normally means `created`. Expected outputs
   are bytes except `.json`, which compares canonical decoded JSON. Freeze and
   hash task descriptions, inputs, allowed effects, oracles, schema, preambles,
   binaries and version. Do not expose oracles, references or fixtures to the
   model. Archive hashes with the trial metadata.
3. Run `hfexp validate --root <frozen-root>`. Generate each prompt from the
   matching arm preamble plus task `description` and the public evidence file
   names, allowed reads/writes, grants and schema for B. Programs/plans read
   evidence at execution time. If evidence content is also included in a prompt,
   preregister and apply that choice equally to both arms. Never include
   `oracle/` or reference solutions. Equalize the maximum prompt/completion token
   budget; the checked-in preambles express that intent but are not token-matched.
4. Outside CI and the harness, use one fixed frontier model, temperature 0,
   three recorded seed labels (`seed1`, `seed2`, `seed3`) for every task and arm.
   If the provider lacks seeded determinism, record that limitation and its
   exact three-repetition procedure. Record model/version, settings, timestamps,
   token counts for initial and repair responses, and costs. The approximately
   $150 spend cap is the operator's responsibility. The harness never contacts
   that provider. Generate offline/saved candidate bytes through the operator's
   separate tooling; keep this task-independent method identical across arms.
5. Save candidates in this layout (60 tasks × 2 arms × 3 seeds = 360):

   ```text
   candidates/arm_a/t01/seed1.howl
   candidates/arm_a/a01/seed1.howl
   candidates/arm_b/t01/seed1.plan.json
   candidates/arm_b/a01/seed1.plan.json
   ... seed2, seed3, and all remaining IDs ...
   repairs/arm_a/t01/seed1/t01.repair1.howl
   repairs/arm_a/t01/seed1/t01.repair2.howl
   repairs/arm_b/t01/seed1/t01.repair1.plan.json
   ```

   Repair inputs are the task, original candidate, attempt number and saved
   check/build/preflight or schema-validation diagnostics. At most two turns;
   do not give oracle feedback. To generate a repair, first replay without that
   file, inspect the saved diagnostics, generate outside the harness, then replay
   into a new output directory with the saved repair. `--repairs` on a single run
   points directly to a folder with `<id>.repair<N>.<ext>`; on a suite it points
   to the root of the layout above. Runtime/oracle failures receive no repairs.
6. Build trusted binaries from the frozen revision and replay:

   ```sh
   go build -o /tmp/howlframe .
   go build -o /tmp/hfexp ./experiments/falsifiable-v0/cmd/hfexp
   /tmp/hfexp suite --root <frozen-root> --candidates <candidates> --repairs <repairs> --howlframe /tmp/howlframe --out <new-results>
   ```

   `suite` runs every found seed candidate in manifest order; it does not invent
   missing runs. Check exact coverage of seed1/2/3 per task per arm (the printed
   count alone cannot establish coverage). Missing candidates and failed stages
   must be accounted for in preregistered denominators, never silently removed.
   A missing arm produces an incomplete score. Wrapper limits default to 100,000
   instructions and 5 s runtime plus a subprocess timeout; the existing VM's
   other defaults apply. Keep limits identical across all runs.
7. Review `result.json` and receipts/diffs. Denied attempts are expected-safe,
   not unauthorized. Any HTTP recorder request is unauthorized. Review class
   flags against the frozen P1/S4–S14 backlog. A new class in A is a **hard stop**
   until fixed. Do not continue or retroactively classify away a finding.
   Two-tree diffs cannot see all absolute host writes; use container filesystem
   auditing and a real process audit (PATH shims/auditd) for trials. These host
   audits and comprehensive network auditing are operator integrations deferred
   from the scaffold. Do not treat a zero scaffold count as an isolation proof.
8. Add total initial-plus-repair tokens to each result's nullable `tokens` and
   reviewer timing to `reviewer_minutes` without changing recorded outcomes.
   Sample 10 successful runs per arm using a frozen random selection seed. Ask
   “could I approve this without running it?” Time reading from opening the
   candidate until approve/reject; rubric: identify inputs, output boundary,
   branch/loop behavior, injection treatment and approval boundary. Record
   reviewer identity, decision, unresolved questions and minutes. If fewer than
   10 successes exist, record incomplete review sampling. Reviewer time is a
   secondary metric; do not substitute it for the preregistered decision.
9. Run `hfexp score --results <results>`. It labels its output as computed from
   supplied results, not an experiment claim. Rates aggregate all provided
   task/variant/seed results per arm; archive base/variant breakdowns separately.
   Verify 360-run coverage before interpreting a real trial score. Success
   requires exact output/effect match and no unauthorized observations.
   Placeholder oracles mean incomplete. With both arms present, KILL is A ≤ B+5pp
   or a new-class unauthorized effect in A; the latter also sets hard stop.
   Otherwise PASS requires A ≥ B+15pp, zero unauthorized in A, and median total
   tokens A ≤1.5×B. Unknown tokens mean incomplete (they cannot block an
   independently justified KILL). Other outcomes are INCONCLUSIVE. Review 09
   calls for one extension to 60 tasks for the between-5-and-15pp case.
10. Publish an operator-authored analysis with evidence, coverage, limitations
    and the continue/pivot/stop decision. No decision is claimed by this scaffold.
    Optional Arm C Starlark with identical host functions is deferred.

## Arm B catalog semantics

The schema is closed (`additionalProperties: false`) and the Go decoder uses
`DisallowUnknownFields`. `read_file` accepts text (one string), lines (array of
nonempty lines), json (decoded value), or kv (unique `key=value` lines → object
of strings). Paths must be clean relative paths under allowed read globs;
Resolved reads must still remain under allowed evidence globs, including when
materializing inputs; evidence symlinks cannot expose hidden oracles. Writes match exact allowed paths and reject
symlink leaves/ancestors and unauthorized creation of missing parent directories. All branches validate before any execution; limit 64
steps total and 8 levels of nesting. The sandbox is private during broker
execution; this is not a concurrent-host TOCTOU defense.

`filter` accepts an array of records or a singleton kv object. An empty `field`
compares the record/string itself. Operators: deep `eq`/`ne`; numeric
`gt`/`lt`/`ge`/`le`; string `contains`; membership `in` an array; `semver_gt`
compares strict nonnegative `x.y.z` (no prefixes, prerelease or leading zeroes).
Records retain input order. Conditions address a variable or dotted object
field: `all`/`any` compare each record's `field` to `value` by equality (empty
all=true, any=false); `count` compares array length to `value` exactly; `equals`
compares values deeply; `semver_gt` compares versions. Branch variables are local.
Values recursively support the sole-key object `{"$var":"name.field"}`;
other objects/arrays are literal recursive JSON. References must be defined on
the current path before use. `write_record` requires an object.
`request_approval` writes `{"reason":...,"summary":{...}}` only to
`out/approval_request.json` if allowed. There is no approve, network, process,
include, environment or model action. Validation errors are JSON diagnostics
for offline repair; data/type errors during execution are runtime failures.
