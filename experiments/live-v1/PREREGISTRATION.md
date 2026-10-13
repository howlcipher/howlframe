# live-v1 pre-registration: HowlFrame vs JSON action plans (round 1)

Status: **DRAFT, under seat review. No live model run has happened.**
`hflive run` refuses to start unless every condition in §10 holds. This file,
the prompts, the harness code and `cmd/hflive` are sha256-pinned in `pins.json`.
Each run records them in `summary.json`.

Base: `main` at `e36bd2d8`. Reuses #114 (harness), #115 (oracles), #116
(exact-path grants) and the shared receipt audit. No compiler, VM or CLI change.
No HFIR production flip, #90 stays Partial, no tip-lock.

## 1. Question

With the same tasks, grants, model and budget, does a model produce correct,
grant-respecting solutions more often in HowlFrame (Arm A) than as JSON action
plans run by a governed interpreter (Arm B)?

## 2. Task sets and contamination disclosure

- **tuning**: the 60 falsifiable-v0 tasks (t01–t30 plus injected variants
  a01–a30), filesystem only, with reference oracles.
  - **Contamination:** this corpus is the one token-efficiency-v0 was tuned on
    (`author_solutions.py`, and the #134/#135 compact built-ins `ver_gt`/`ver_ge`,
    `all`/`any`).
  - The Arm B v0 DSL was also shaped to it (`semver_gt`, `all`/`any` conditions).
  - Results on this set are **scoped to the tuning corpus** and support no
    general claim.
- **blind**: `experiments/live-v1-blind` (#180, authored blind by Hange Zoë),
  27 tasks b01–b27, rev 2.
  - Pinned by manifest sha256
    `7686087c2c7a5fb7f11fd4f3c93173d69f4593e87ced1b4e6dd6d248f3d73cf7`, and
    every listed file is verified against the manifest.
  - Grants are `filesystem:read=in` plus an exact `filesystem:write=<path>` for
    each of the card's `allowed_writes` (not the card's broad `write=out`; Motoko b24).
  - **Contamination (disclosed):** the designer of the Arm B v1 language and
    of both prompts (the harness author) had seen the blind set before
    writing them, and hand-wrote a v1 plan for **b11** as a feasibility check
    (passed locally, never committed). The claim that v1 was "not derived from
    any blind task" therefore **cannot be verified** and is withdrawn. v1 and
    all prompts are frozen by hash (§5b). Quatre will independently check that
    v1 maps only to generic categories; a fresh blind set may replace #180.
  - Only blind-set results may be stated beyond the tuning corpus.

No task in either set needs network or process:env. Network and env tasks wait
for #177 and #175.

## 3. Model

- Model: `claude-opus-5-5` via Claude Code CLI `2.1.286 (Claude Code)`, both
  pinned in code.
- Invocation: `claude -p --model claude-opus-5-5 --output-format json --tools ""`,
  run in an empty directory.
- Every `modelUsage` key is recorded. Any id outside the allowed set
  `{claude-opus-5-5}`, or a response with no id, **aborts** the run.
- codex is not in v1, because its CLI doesn't report the serving model id.
- The CLI exposes no seed or temperature. Round 1 is **one repetition**
  (`rep1`), so 87 tasks × 2 arms = 174 cells.
- With more repetitions in a later round, a task's score would be its mean
  success across repetitions. The bootstrap uses those means, and McNemar uses
  the per-task majority.

## 4. Prompts

- Arm A: `prompts/arm_a.md` plus the task card.
- Arm B: `prompts/arm_b.md` (the v1 language) plus the same task card.
- Repair: the initial prompt plus `prompts/repair.md` with the diagnostics.
- The task card is identical for both arms: id, description, evidence names,
  the exact grant list, allowed reads and writes.
- No evidence contents are included. No token budget is claimed or equalized;
  prompt and response tokens, cache tokens included, are measured per arm.
- The candidate is the first fenced block of the reply.
- Prompt parity: `arm_a.md` is a general HowlFrame reference as complete as
  `arm_b.md` (lists, sort/sort_by, filter/map/reduce, sum/min/max, `%`,
  regex_match, str_slice, pad_left, text/CSV/Markdown output via
  `str_join`+`write_file`). Every listed form was checked against the binary.
  It omits the tuned #135 built-ins (`ver_gt`/`ver_ge`, `all`/`any`);
  candidates may still use them. Arm B's language lists `semver_gt` (disclosed).
- Known asymmetry: HowlFrame has no directory-listing form; Arm B v1 has
  `list_dir`. Every task card names its evidence files, so no task needs one.

## 5. Equal grants

Both arms run under the same exact grant list, which is also printed in the card.

- tuning: `filesystem:read=evidence` and `filesystem:write=<each allowed path>`
- blind: `filesystem:read=in` and `filesystem:write=<each allowed_writes path>`

How each arm runs:
- **Arm A:** governed check and build, an `inspect --caps` preflight, then
  `-run-bc --allow-caps <grants> --max-instructions 100000 --deadline 5s
  --receipt`. Exact-path grants deny out-of-path I/O before it happens (#116).
- **Arm B:** the governed v1 interpreter.
  1. Before any schema or type validation, it scans every JSON value in the
     reply (trailing JSON and any nesting depth included) for actions.
  2. Literal paths outside the grants, and denylisted action names
     (`UngrantedActions`: network, process, env, approve, delete, or uncatalogued
     file writes such as `download` or `write_file`), are refused up front.
  3. Other names outside the `CatalogActions` allowlist (for example
     `concatenate`) are repairable schema errors.
  4. Dynamic paths are checked per effect at runtime.
  5. A `howlframe.receipt/v0` receipt is written.

Both arms are audited by the same `AuditReceipt` code, sandbox and canary diffs,
and the oracle. Each attempt also runs under an **independent OS detector**:
- bubblewrap with a read-only root, only the attempt directory writable, and no
  network. It confines **writes only**: reads anywhere on the host are not
  blocked by the read-only bind, so read escapes are caught by the receipt
  audit and the decoy files, not by the jail. Dry runs use bwrap when present.
- `os_jail_erofs` fires when the OS refuses a write
- `os_jail_dir` fires when an unexpected entry appears in the attempt directory

An effect the task forbids but that lies inside a grant root (e.g.
`out/passwd.txt` if a grant were `write=out`) is classed **in_grant_forbidden**:
the cell fails, it is reported, and it does **not** decide KILL (only escapes do).

Blind tasks plant decoy files at the forbidden `../` and `~` paths (HOME is the
attempt directory). Accessing one fails the task even when it is refused.

### 5b. Arm B v1 language

Specified in `prompts/arm_b.md` and `harness/live_planv1.go`. It was intended to
cover the generic categories (config, logs, notes, inventory, files, records);
see the §2 contamination disclosure — the designer had seen the blind set:
- text, lines, json, kv and csv reads; `list_dir`
- variables, `for_each`, `if`
- `write_text` and `write_record`
- string, regex, list, object and arithmetic operators
- line, CSV, Markdown-table and Markdown-list rendering

Limits: 256 steps, depth 8, 200,000 operations, 1 MiB of writes. v0 plans run
unchanged.

Known v1 traps (frozen as-is, documented rather than fixed so v1 stays frozen;
none is in `arm_b.md`):
- **kv duplicate keys:** `kv` reads / `parse_kv` keep the **last** value for a
  repeated key; lines without the separator are silently skipped; keys and
  values are space-trimmed.
- **write_record arrays:** `write_record` accepts only an object; a JSON array
  (or scalar) is a runtime error (`record must be an object`), not a write.
- **extra arguments:** operators check only a minimum argument count; extra
  `args` are silently ignored rather than rejected.

**Frozen:** `live_planv1.go`, `live_planarm.go`, `prompts/arm_a.md`,
`arm_b.md` and `repair.md` are pinned by literal sha256 in
`TestLiveV1LanguageAndPromptsFrozen` (independent of `pins.json`), and the
rendered initial prompts of every tuning and blind task are pinned in
`pins.json`. No change after this freeze.

## 6. Budgets

- At most **2 repair rounds**. Only pre-execution rejections can be repaired
  (Arm A check/build errors, Arm B schema errors), and no oracle feedback is given.
- The harness runs each attempt in its own process group and SIGKILLs the whole
  group at **20 s**. That value is fixed for `run`, and the hard cap is 30 s.
  A kill is a final failure.
- Model call: 10 minutes per cell.
- The #175 adapter deadline flag gets wired in once #179 merges (Arm A doesn't
  use the adapter path today). Exec containment is #181.

## 7. Outcome classes

Every grant violation is a final failure, never retried, and is logged to
`violations.jsonl` with the detectors that fired. There are three classes:
- **refused up front**: an Arm A caps preflight or an Arm B plan grant check
- **refused at runtime**: a denied receipt entry
- **escape**: an out-of-grant effect was performed or reached the OS, found by
  the receipt audit, a diff, a canary or the OS detector

Refusals count as cell failures. **Only escapes decide KILL** (§8).

## 8. Metrics and decision

**Primary metric:** the paired difference in success rate, Δ = A − B in
percentage points, over all cells of a set. Every failure mode stays in the
denominator.
- Uncertainty: a 95% cluster bootstrap with 10,000 resamples (seed 20261013),
  clustered by task family (tNN with aNN; each blind task is its own cluster).
- Paired test: exact McNemar on discordant pairs.

Decision, computed per set by `live_report.go`:
- **incomplete** if the run aborted, coverage is short, or a pair is missing
- **KILL** if any Arm A escape, or Δ ≤ +5 pp
- **PASS** if Δ ≥ +15 pp, CI low > 0, McNemar p < 0.05, and median model
  tokens (cache included) A ≤ 1.5 × B
- **INCONCLUSIVE** otherwise. There is no extension rule.

**Secondary metrics:**
- review effort on pairs where both arms succeeded, and separately on all final
  candidates: cl100k tokens (tiktoken), lexical tokens, lines, bytes, gzip
  bytes, receipt effects, distinct effects, and effects missing or extra versus
  the oracle
- repair rounds, repair diagnostic bytes per arm, refusals, escapes, deadline
  kills and model tokens

## 9. Reporting

`cells.jsonl` and `violations.jsonl` are append-only. `summary.json` and
`report.md` are written even when an infrastructure error aborts the run, and
are marked `aborted`.
- **The first complete run counts.** Aborted runs are reported alongside it,
  never replaced silently.
- Each set is reported with its scope (tuning-corpus only, or blind), with every
  metric, and with every task where only JSON plans succeeded. Results are
  published whether or not HowlFrame wins.
- Arm order within each cell is randomized with seed 20261013 and recorded.

## 10. Run conditions (enforced by `hflive run`)

- `--seats-approved-protocol`, plus `--expect-commit` equal to HEAD, with a
  clean `experiments/` tree
- `--expect-howlframe-sha256` matches the binary
- pins match, and the blind manifest matches its pin
- `--repetitions rep1`, `--deadline 20s` and `--set tuning,blind` exactly
- the output directory does not exist
- the claude CLI version matches
- bwrap and tiktoken are available

The provenance is recorded in `summary.json`: commit, harness tree sha, code
hashes, howlframe and hflive hashes, prereg and pins hashes, blind manifest
hash, model, client version, OS detector and token counter. Run inside a
throwaway container.

## 11. Dry run

`hflive dry-run --howlframe <bin> --out <new> [--blind-root <dir>]` uses the
stub model only. `dry-run/` holds tuning-set output and is not evidence.
