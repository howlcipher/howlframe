Created [heldout/test_boundaries.py](heldout/test_boundaries.py) with **24 tests** covering validation and metric boundaries, ordering, CLI forms and errors, window edges, and a 1,200-record input. [HELDOUT_NOTES.md](HELDOUT_NOTES.md) records coverage, ambiguities, and the requested attestation.

The brief’s collection check reported 24 tests, and the test file compiled. I did not run the tests against an implementation.

**Ambiguities:** service-name collation for non-ASCII text; whether whitespace-only service and client IDs are trimmed; duplicate JSON keys and non-standard JSON constants; whether SLA checks use rounded averages; and behavior when input and output paths match or options repeat.
