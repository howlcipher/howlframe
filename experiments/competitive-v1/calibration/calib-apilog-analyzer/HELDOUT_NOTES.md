# Held-out test notes

## Files

- `heldout/test_boundaries.py` contains 29 tests for record validation boundaries (including Revision 1 numeric rules), status classes, ignored fields, SLA threshold equality, window boundaries and ordering, client ranking, CLI option forms and failures, and a 1,200-record input.

## Ambiguities

- The contract specifies service names are sorted alphabetically but does not define collation for non-ASCII names. Tests use ASCII service names.
- It does not define whether whitespace-only `service` or `client_id` values count as non-empty after trimming. The stated constraint is length at least one, so the boundary test treats a single space as non-empty.
- It does not specify duplicate JSON object key handling. Revision 1 explicitly classifies `NaN`, `Infinity`, and overflowing exponent forms as invalid numeric values, and tests that rule.
- It does not say whether SLA comparisons use the unrounded averages or the rounded metric values. Tests cover exact thresholds without a rounding-dependent case.
- It does not specify behavior when input and output paths refer to the same file, or when option flags are repeated.

## Revision 1

Updated numeric validation coverage to accept whole-valued JSON numbers (including exponent notation), while rejecting fractional values, non-number types, non-finite values, negative values where prohibited, and integers above `2^53 - 1`. The inclusive safe-integer ceiling is checked on timestamp, latency, and bytes fields. The existing directory-input test already checks exit code 2, as required by the revision. No new acceptance test scenario was duplicated.

## Attestation

I wrote these tests new for this purpose and did not see any implementation of the task.
