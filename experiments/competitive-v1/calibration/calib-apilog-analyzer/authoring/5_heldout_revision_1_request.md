# Revision request 1 for the held-out tests

The task author revised the contract after your held-out tests exposed two ambiguities. `task/prompt.md` and
`task/acceptance/` are now the revised versions; `contract_change.diff` shows exactly what changed in `prompt.md`:

1. Integer fields accept any finite JSON number with a whole-number value, so `1.0` is now **valid** (as 1). Fractional
   values, non-finite numbers, values above 2^53 - 1, strings, booleans and null are invalid.
2. An `--input` path that is a directory exits 2 (your existing test already expects this).

Update `heldout/` so every test matches the revised contract: change
`test_numeric_fields_reject_floats_strings_and_booleans`, and add held-out coverage for the new rules (for example,
exponent forms, values exactly at 2^53 - 1 and just above it, negative whole numbers such as `-1.0`). Avoid duplicating
the new acceptance tests. Keep everything else in the brief (`BRIEF.md`). Update `HELDOUT_NOTES.md` with a short
"Revision 1" section, check that the files still import and are collected, and print the new test count and what changed.
