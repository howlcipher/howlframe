# Pilot candidate pool (protocol §3.2): authored and checked, **not frozen**

28 candidates, 7 for each of D1–D4, written by Antigravity (`gemini-3.8-flash-high`) on 2026-10-25 in two sessions
(D1+D2, D3+D4) from `authoring/1_pool_brief.md`; the revision request and the author's reports are beside it. Each candidate has a language-neutral
contract (`prompt.md`, plus `baseline_spec.md` for D4), acceptance tests, a workload generator, and two independent
screen implementations (`screen/python`, `screen/javascript`). The screens prove the contract and tests are
consistent; they are never shown to agents.

No paid call was made: the author ran on the Owner's Antigravity subscription.

## Check (`python3 -m cv1.check_pool`, pinned toolchains, scoring jails)

`POOL_CHECK_2026-10-25.json` is the full report (manifest digest `990e2601…`). All 28 candidates pass:
* metadata, contract and test files are present;
* both screens complete every acceptance test in fresh jails;
* no canary fires;
* no delegation flag is raised;
* each domain has at least 6 candidates and room under the authority cap.

The largest word 5-gram overlap between any prompt and the HowlFrame corpora (`benchmarks/`, `falsifiable-v0`,
`live-v1-blind`, the token-efficiency sets) is 0.2%, which is shared boilerplate.

Revision during authoring: on the author's host, 4 D3 candidates passed, but in the jails they failed. Their tests made
the program launch `python3`, which exists only in the Python arm's jail. That would have made those tasks impossible
in the other three arms. The author changed every child process to programs present in all jails, named by their
canonical paths: `/usr/bin/dash`, `/usr/bin/coreutils`. HowlFrame refuses symlinked programs by design, and most of
`/usr/bin` in the base is symlinks.

## Authority labels (the author's; the §3.2 step 3 cap allows at most 1 of the 4 pilot tasks)

`d1-package-sbom`, `d1-rbac-audit`, `d2-feature-flag`, `d2-rate-limiter`, `d3-sandbox-exec-policy`,
`d3-token-vault-agent`, `d4-api-key-permission-manager`, `d4-rbac-audit-filter`. Each domain has 2, and the reasons are
in `AUTHOR_NOTES_*.md`.

## Still required before the freeze (not done; none is an agent's call alone)

1. **Reviewer audit of the labels and the corpus check (§3.2 steps 1 and 3).** It must be done by a reviewer other
   than the author. Antigravity wrote the pool, so it cannot review it, and Claude and Codex are trial providers.
   That leaves the Owner or another human.
2. **Owner yes for the freeze.** The freeze commits this pool's manifest digest (from `check_pool`), then sets the
   beacon timestamp at least 24 hours later (§13.3).
3. **After the draw, for the 4 drawn tasks only:**
   * held-out tests by a different author and provider from the acceptance tests. Both trial providers are excluded,
     which leaves a human or another provider: an Owner decision;
   * references in all four languages;
   * workload sizing and pins;
   * D4 baseline implementations in all four languages by one author.

## Files

* `<id>/`: the candidates.
* `SCREEN_LOG_*.md`: every candidate considered, including the dropped ones, with reasons.
* `AUTHOR_NOTES_*.md`: variety, labels, attestation, and the revision.
* `DIAGNOSTIC_RUBRIC.md`: the 1–5 rubric for blind grading of first errors (protocol §6), written by the same author.
