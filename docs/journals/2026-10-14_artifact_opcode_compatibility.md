# Release artifact opcode compatibility

Unchanged ChangeOps e039fb4 and its release-compiled Frame71a policy failed under
current d7b623c Frame: missingfunction bytes_to_string. Old CONVERT enum ordinal
now identifies CALL because later instructions were inserted before it. Gob
retained both the old ordinal and stable OpString; decoding privileged the ordinal.

Options: freeze all numeric ordinals/bump wire ABI (broad migration), require
recompilation (leaves advertised legacy-envelope consumption defective), or use
stable names at the existing deserialization boundary as JSON already does.
The smallest correction chooses known names before unchanged structural checks.
Numeric-only artifacts retain current structural validation; names never grant
capabilities. Unknown names, missing calls and invalid jumps remain rejected.

Regressions cover both envelope versions, main/function instructions, actual
conversion output and named filesystem denial. An unchanged older policy now
returns REQUIRE_APPROVAL through the public ChangeOps command; no approval or
execution was performed. Native host exec-fixture portability failures remain
separate; full supported CI is required before source acceptance.
