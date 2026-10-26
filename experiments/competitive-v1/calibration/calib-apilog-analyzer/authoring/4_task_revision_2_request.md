# Revision request 2 for calibration_task

A second author (Codex, OpenAI) wrote held-out tests from your `prompt.md` alone, without seeing your references. Scored in
the sandbox, the Python reference passes all 24, while the Go and JavaScript references each fail the same 2. Both
failures come from places where the contract can be read two ways. The contract is yours to settle; please decide each
point, state the rule explicitly in `prompt.md`, and make all three references follow it. Change nothing else.

1. **Is `1.0` an integer?** `prompt.md` says booleans and floats are not valid integers for `timestamp`, `status`,
   `latency_ms` and `bytes_sent`. The held-out test treats a record whose field is written `1.0` as invalid. Python's
   `json` module distinguishes `1.0` from `1`; Go only does if it decodes with `UseNumber`; Node.js's `JSON.parse` cannot
   distinguish them at all, so a lexical rule ("written without a decimal point or exponent") would force the
   JavaScript arm to hand-write a JSON number scanner. The experiment compares languages, so the operator recommends a
   rule every language can meet with its standard JSON parser, for example: "a JSON number whose value is a whole number
   (so `1.0` is treated as `1`; `1.5` and `1e400` are invalid); strings and booleans are invalid". Choose that or another
   rule that is equally implementable in all three languages, and say it precisely. Also state what happens to a
   whole-number value larger than 2^53, or rule such values invalid, since JavaScript numbers lose precision there.

2. **Input path that is a directory.** The contract says exit 2 when `--input` "does not exist or cannot be opened for
   reading". The held-out test passes a directory and expects exit 2; Python's reference exits 2, Go's exits 0 and
   JavaScript's exits 1. State the rule explicitly (the operator suggests: "does not exist, is a directory, or cannot be
   read" → exit 2) and make the references agree.

Then re-run the acceptance tests against all three references, update AUTHOR_NOTES.md (a short "Revision 2" section), and
print a short report of the rules you chose.
