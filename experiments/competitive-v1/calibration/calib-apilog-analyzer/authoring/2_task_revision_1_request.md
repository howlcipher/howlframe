# Revision request 1 for calibration_task

The harness operator checked your task with the experiment's own scorer: each reference was copied into a fresh
sandbox with the pinned toolchains (CPython 3.12.15, Go 1.25.14, Node.js 22.23.4) and scored. All three references
passed all 16 acceptance tests. Two things need changing; please make only these changes.

1. `references/go/build` hardcodes a GOCACHE path inside this authoring directory. Remove that; the build must not
   depend on any path outside `references/go/`. A plain `go build -o app main.go` from the script's directory is enough
   (the scoring sandbox provides a writable HOME). Do not commit the built `app` binary (it was removed).

2. Workload sizing. Protocol §3.3 requires the workload to be sized so the reference runs 0.5 to 5 s excluding process
   startup; the operator applies this to every baseline reference. Measured in the scoring sandbox at 100,000 records
   (median, startup subtracted): Python 0.71 s, Go 1.14 s, JavaScript 0.28 s (below the range). Choose a record count
   that puts all three inside 0.5–5 s with margin at both ends (time scales roughly linearly with records).

   The workload file will no longer be committed; it is generated. Make `generate.py`'s defaults produce the chosen
   workload (keep seed 42 and the output name `requests.jsonl` next to the script), then set task.json's workload to:

   ```json
   "workload": {"args": ["--input", "/workload/requests.jsonl", "--output", "/tmp/report.json"], "stdin": null,
                "generate": {"command": ["generate.py"], "files": {"requests.jsonl": "<sha256 of the generated file>"}}}
   ```

   The harness runs `python3 -I generate.py` inside `workload/` with CPython 3.12 and refuses to run if the output's
   SHA-256 differs from the pin, so compute the pin from a file generated exactly that way (`cd workload && python3 -I
   generate.py && sha256sum requests.jsonl`). Then delete the generated `requests.jsonl`.

Update AUTHOR_NOTES.md (record count, and note the sandbox timings above came from the operator), re-run the
acceptance tests against the three references, and print a short report of what you changed.
