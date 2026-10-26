# Final handoff: post-merge verification and competitive pilot readiness

Mission dates: 2026-10-23 to 2026-10-24. Agent: Claude Opus 5.5 (`claude-opus-5-5`, Claude Code 2.1.309).
Baseline: HowlFrame `056fbf89e7477637e3e96acd93f3d14870dbc3fc`, HowlBoard `0b9236bc637fee00e4c400a3304230e42f0c7c49`.
These reports were written outside the repository and committed here, documentation only, at the Owner's request on 2026-10-24
("merge all when complete", scoped to this reports PR; draft #213 stays unmerged).

## Deliverables

| Document | Contents |
|---|---|
| `HOWLFRAME_CURRENT_VERIFICATION.md` | SHAs, CI evidence, 16 commands with exit codes, M1/M2/M3 assessment, security regressions, findings F1–F4 |
| `HOWLFRAME_COMPETITIVE_READINESS.md` | Protocol status, gaps G1–G12, infrastructure inventory, non-billable checks, approval needs |
| `HOWLFRAME_MODEL_SELECTION.md` | Candidate IDs and prices with sources, recommended configuration, parity settings, undisclosable differences |
| `HOWLFRAME_EXPERIMENT_BUDGET.md` | Empirical token profile, 4-run and 96-run estimates, caps, enforcement design, stop condition |
| `HOWLFRAME_OWNER_APPROVAL_PACKET.md` | Decisions 1–10 with recommendations |
| `evidence/` | All logs (`logs/summary.txt`, `logs/summary2.txt` index them), run scripts, usage-statistics scripts, beacon probes |

## Verified (observed)

* Both default branches are unchanged since the historical references. Required checks are green on the merged HEAD:
  `build`, `acceptance`, `race` (and `latest-go`).
* In a fresh clone: gofmt, build, vet, `go test`, `go test -race` and `scripts/ci_acceptance.sh` all exit 0. M1: 35 + 29 + 32. M2: 101 frozen
  + 13 DX + adversarial, review, m10 and restart suites.
* EvidenceDesk `scripts/test.sh` passes against **unmodified current HowlBoard**. That covers the 6 frozen tests, real Chromium 148 with
  XSS, restart persistence, five capability denials and the actual Board consumer on both sanitized fixtures. The adapter path is taken
  only for historical `cd2f2fd`.
* HowlBoard's importer genuinely uses the EvidenceDesk validator: a `use` statement in the compiled importer, a byte-identical vendored module,
  refusal messages that come from that module, and a store unchanged after refusal. `make test` (23 tests) also passes with the **current**
  HowlFrame compiler.
* Security regressions for filesystem races, single-file roots, descendant kill, TicketDesk restart, slow client, HF-05 and env families all
  pass, and all of them sit inside required CI jobs.

## Not done, by design

No paid model call, no trial, no freeze, no merge of #213, no HowlFrame source change, no change to frozen acceptance criteria, no review
cycle, and no software installed. Private HowlPlane ledgers were not read.

## Incomplete or open

* Protocol Revision 3 (G1–G12) is not drafted, pending Owner decision 5.
* Experiment infrastructure (runner, jail, proxy, scorer, schema, beacon draw) does not exist, pending decision 6.
* The task pool and tests need independent authors (decision 8).
* Non-blocking follow-ups: bump HowlBoard CI's HowlFrame pin from `a154ddac` (F1); correct the #213 head in the 2026-10-22 handoff (F2);
  note that `~/.local/bin/howlframe` is the stale 0.1.0 release (F3).

## Precise next authorized action

**None is authorized yet.** The next step is the Owner's answers to approval-packet items 1–6. If they match the recommendations, the
next action is: in a new worktree off `main`, draft protocol Revision 3 and build the experiment-only infrastructure under
`experiments/competitive-v1/`, validated against mock providers only, as a draft PR. Then request one scoped different-provider review of
the Revision 3 deltas. Only then return for the freeze and the dated go-ahead for the $150 feasibility test.
