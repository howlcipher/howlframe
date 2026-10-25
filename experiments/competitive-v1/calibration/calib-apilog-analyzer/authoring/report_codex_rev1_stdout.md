Updated `heldout/test_boundaries.py` and `HELDOUT_NOTES.md`. The suite now has **29 tests**. It accepts whole-number exponent notation and the inclusive `2^53 - 1` limit for timestamp, latency, and bytes; it rejects fractional, non-finite, negative where prohibited, and over-limit values. The existing directory-input test already expects exit code 2.

Import and collection check succeeded: **29 tests collected**.
