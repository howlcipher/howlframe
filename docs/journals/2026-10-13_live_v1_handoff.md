# live-v1 (#178) handoff — 2026-10-13

Branch `live-comparison-harness` (no force-push). Resume from this list.
Code: `experiments/falsifiable-v0/harness/live*.go`, `experiments/live-v1/`.
Box: bwrap 0.12 works (unprivileged); tiktoken at /workspace/tokvenv/bin/python;
claude CLI 2.1.286 logged in; codex dropped from v1 (no served-model id).

Regenerate pins after any protocol/code change:
`go run ./experiments/live-v1/cmd/hflive pins > experiments/live-v1/pins.json`.
Local blind smoke: `git fetch origin pull/180/head:pr180r2`, extract
`experiments/live-v1-blind` to /tmp and pass `--blind-root` to `hflive dry-run`.

## Heinrich (blocking 1–8)
1. Contamination disclosure + blind slot — DONE (blind loader pinned to #180 rev 2 manifest 7686087c; §2 disclosure).
2. Pin harness tree, binary hash, prereg text into summary.json — DONE (Provenance; code + prereg + cmd in pins.json).
3. Reruns/seed selection — DONE in code (fixed --repetitions/--deadline/--set on run, existing out refused for both commands, append-only cells.jsonl/violations.jsonl, aborted summary written). Doc §9 DONE.
4. Decision rule — DONE in code (primary = paired success delta; PASS/KILL/INCONCLUSIVE in live_report.go; McNemar exact + family cluster bootstrap 10k, seed 20261013; no "expand to 60"). Doc §8 and TestLiveDecisionRule DONE.
5. Review effort secondary, paired both-success + all, cl100k via tiktoken — DONE.
6. Prompts: equal-budget claim removed — DONE; pins regenerated.
7. OS detector (bwrap RO root, attempt dir writable, no net; os_jail_erofs / os_jail_dir; detectors logged) — DONE incl. TestLiveOSDetectorCatchesEscapes (skips when bwrap unavailable, e.g. CI).
8. Arm B explicit lists — DONE (CatalogActions allowlist + UngrantedActions denylist). Tests DONE.
Non-blocking: arm order randomized (seed 20261013) DONE; all model-id keys recorded DONE; CLI version pinned DONE; diagnostic bytes per arm DONE; "repetitions" DONE; omitted tuned built-ins justified in §4 DONE.

## Lain (1–5)
Lain's #178 items were not relayed to this session except the #179 note:
wire the #175 AdapterDeadlineFlag into the runner once #179 merges — TODO (Arm A
does not use the adapter path today; harness process-group SIGKILL covers the
harness side; exec containment is #181). Re-read Lain's #178 review and fill in.

## Motoko (1–4)
1. Arm A uses #116 exact-path grants (filesystem:read=evidence, filesystem:write=<each path>; blind uses card grants) — DONE; refusals up front / runtime / escapes reported separately — DONE; only escapes decide KILL, refusals are cell failures — DONE (§7); path-scoping caveat removed from prompt and doc — DONE.
2. Arm B grant check before schema/type (generic scan of every JSON value incl. trailing JSON and any depth) — DONE with tests.
3. Pin harness code, cmd/hflive, binary; fix --deadline/--repetitions at run — DONE.
4. Model id mismatch aborts (ErrInfra) — DONE; codex dropped — DONE; Claude allowed id set pinned {claude-opus-5-5} — DONE (amend before run if Claude Code reports an auxiliary model).
Also: McNemar + cluster bootstrap DONE; repetitions combine as per-task mean, McNemar on majority — DONE (§3); review effort effect counts / distinct / oracle diff / gzip bytes DONE; cache tokens DONE; round 1 = single repetition (rep1) DONE.

## Arm B extension + blind adapter
- Arm B v1 action language (`live_planv1.go`): read_file text/lines/json/kv/csv, list_dir, set, append, filter, for_each, if (expr or legacy cond), write_text, write_record, request_approval; closed $op set (strings, regex, lists, objects, arithmetic, render_lines/csv/md_table/md_list). v0 plans still run. DONE: prompts/arm_b.md describes v1 (it is the reference); tests for rendering and dynamic-path runtime refusal.
- Blind adapter (`live_blind.go`): loads task_card/workspace/in/expected/oracle.json, verifies manifest + every file hash, plants decoys at forbidden ../ and ~ paths, compare = byte-exact after stripping trailing LFs / JSON by value, forbidden access fails even when refused. DONE; pinned to rev 2 manifest 7686087c. Stub dry-run over both sets passes (blind has no references, so all fail as expected).

## Remaining
- Lain's #178 items 1–5: not relayed here; reconcile.
- #175: hflive now takes -adapter-deadline via vm.AdapterDeadlineFlag (cap 30s, fixed at 1s for run, recorded). The howlframe CLI itself does not register the flag yet (#175 reopened); Arm A does not use the adapter path.
- Main merged (6c969620, merge commit). Blind set loads from experiments/live-v1-blind (7686087c); TestLiveBlindSetLoadsAndCompares covers all 27 tasks. Feasibility: a hand-written v1 plan for b11 (text output) passes locally; not committed (no blind solutions in repo).
