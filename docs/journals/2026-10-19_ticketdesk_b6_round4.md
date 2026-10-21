# Ticketdesk B6 independent delta check, 2026-10-19

Reviewed commit `7092bcff`; reviewer: independent final reviewer (Codex CLI session; no implementation role).
No product changes. Frozen paths unchanged. Executable evidence and commands are in Round 4 of
`milestone2/MILESTONE2_DECISION.md` and `milestone2/ticketdesk/evidence/round4_2026-10-19/`.

The independent stdlib storm seeded 1500 tickets through HTTP and killed 25 active DELETE loops at default
runner ceilings. Recovery listing and all survivor GETs passed in every trial. Eleven caught unindexed
readable records, consistent with unindex-before-delete. Twenty-four runs completed all survivor deletion
checks; run 24 timed out during deletion after recovery reads passed. The script reports no exact DELETE
id, and the cause is unclassified. Exit 1 means validation incomplete: NOT ACCEPTED. No observed 500 or
inconsistent durable state; do not report the timeout as proven recurrence of B6 or a new product blocker.

Crafted dangling ids across bucket boundaries are skipped and do not block subsequent writes or walks;
GET/DELETE return 404 and leave stale entries (new minor m18). Crafted unindexed records remain readable
and DELETE repairs them. m12, m15–m17 remain. Full clean-repro: 101 acceptance, 9 regression, 13 DX pass.
Go uncached suite: 76 packages ok, exit 0. VM hunk preserves capability attenuation and AllowedCaps.
The first probe setup incorrectly used unsupported status `closed`; corrected to `done` before the storm.
