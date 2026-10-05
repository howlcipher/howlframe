# Mission journal: AI-native architecture and product-direction review

- **Date:** 2026-10-05. All times are ET (America/Detroit).
- **Requested by:** William Elias, via EM Rintaro Okabe.
- **Executor:** Grok Bot, working only with local tools. Cloud agents were
  out of usage.

## Ground rules followed
- Worked from the latest `origin/main`: fetched and confirmed
  `d496d9721122681b0adbb5d27764f55a21c43a3d` (#103).
- Worked in a fresh clone under `/tmp/howlframe-ainative`. The user's
  checkout at `/workspace/howlframe` was not touched.
- **Branch:** `rintaro/ai-native-language-review`.
- **Hard nos respected:**
  - no production `-compile-bc` / HFIR flip;
  - #90 left Partial;
  - the `4d74dbcf` tip-lock was not retaken;
  - no DOM APIs, no language redesign, no artifact format change.
- Draft PR only. It will not be undrafted or merged by the executor.

## Timeline
1. **About 19:16.** Cloned, fetched, and confirmed the tip. Ran the
   baseline `go test ./...`: 38/38 packages ok, 49.6 s, exit 0. Log:
   `/tmp/howlframe-ainative-gotest-baseline.log`.
2. **Current-state inspection (before polling any agent).** Read the
   README, AGENTS.md, roadmap and status docs, `howlframe.go`,
   `capability`, `opcode.go`, `artifact.go`, `construct.go`, `hfir/*`,
   `vm.go`, the apps, the contracts, and `git log`. Measured the size:
   ~22.7k non-test Go LOC, ~27.9k test LOC, 434 commits since 2026-07-23.
   Registry counts: 87 / 9 / 11 constructs, 69 opcodes.
3. **About 19:18. Probes** (`/tmp/hf-ainative-probe/`). A Python recording
   listener ran on 127.0.0.1:11434.
   - `confidence`, `neural_circuit`, and `ephemeral_circuit` sent requests
     under an empty grant on `-run-bc`. `ephemeral_circuit` also issued
     `api/create` and `api/delete`.
   - Interpreter `confidence` and `lazy_synthesize` sent requests under an
     empty grant on `-run`.
   - The listener was killed afterward.
4. **Agent workspace set up.** Built `/tmp/hf-ainative-agents/` with the
   common prompt, role files A to F, and isolated clones `repo-{A..F}` at
   `d496d97`. Prompts contained no executor findings.
5. **Agents launched in parallel.**
   - **Codex** (`codex exec -s workspace-write`, no dangerous flags) for
     A, B, D, and F.
   - **AGY** (`agy -p --mode plan`):
     - Claude, Opus, Sonnet, and GPT-OSS attempts either exited early (the
       headless `run_command` denial) or hit 429 quota limits.
     - Added a box-local `~/.gemini/antigravity-cli/settings.json` (about
       19:24) with read-only `command(...)` allow rules. **This changes the
       box only, outside the repo.**
     - Added `REVIEW_CONTEXT_PACK.md` (raw files, unmodified) to the AGY
       clones, with a no-shell prompt.
     - Gemini 3.1 Pro High ran C. Gemini 3.8 Flash High ran E.
   - **Claude CLI:** weekly limit until about Oct 7, 2 pm ET. Not used.
6. **About 19:25. More probes.**
   - `mem.howl`: string doubling under the default budget, zero grants.
     Reached ~2.06 GB RSS in 15.6 s (S4).
   - `rec.howl`: recursion with `--max-instructions 2000000000` ended in a
     Go fatal stack overflow (S5).
   - `andenv.howl` (about 19:28): reproduced the `and`/`or` divergence
     that reviewer A reported (S10).
7. **Fix (commit 1, `fbebaef`).**
   - **Change:** `network` capability on the three opcodes, in
     `ForConstruct`, and on the interpreter `lazy_synthesize` check.
   - **Tests:** `internal/vm/model_call_capability_test.go`, a recording
     probe with a granted positive control. Also additions to
     `TestCapabilityGatePerKind` and `TestForConstruct_Known`.
   - **Red check:** the new tests fail on the old code.
   - **Bookkeeping:**
     - bugs.md #57 and a change_log entry;
     - journal `docs/journals/2026-10-05_model_call_capability_gates.md`;
     - the 3 opcode rows in `bytecode_reference.md`;
     - regenerated `construct_coverage.md`.
   - **Checks:** `gofmt` and `go vet` clean, build ok, `go test ./...`
     38/38 ok, exit 0. Log: `/tmp/howlframe-ainative-gotest-fix.log`.
     `scripts/test_seo.py` and the `benchmarks/v2` unittest also pass.
8. **Agent outputs arrived** (19:23 to 19:36). Each was graded only after
   collection (04).
   - B, D, and F independently confirmed the S1 hole.
   - D found that the `release_authority` output breaks on a quoted
     `target`. The executor escalated this to **forging
     `"decision":"ALLOW"` while the VM decided `DENY`**. Python
     `json.loads` keeps the last duplicate key. Repro:
     `/tmp/hf-ainative-probe/inj2.json` → `ra2.out` (S16).
   - F found partial effects in `action_executor` that contradict the
     "failure atomicity" docs. The executor confirmed the ordering in
     source (S17).
   - C made false VERIFIED claims:
     - the capability names;
     - a token advantage over Python (the CSV shows otherwise);
     - that multi-node repair is "absent" (`hfir-semantic-repair/v2`
       exists).
   - C also recommended the HFIR flip. Rejected under the hard nos.
   - E's output is truncated, and its citations are context-pack offsets.
9. **Docs written.** `docs/ai-native-language-review/00` to `11`,
   `FINAL_REPORT.md`, this journal, and `raw/`.

## Deliberately not done
- **Demo-app JSON escaping (C7).** It changes app output formatting, which
  is a consumer-visible behavior change, so it is planned rather than
  done.
- **Resource limits (C4).** The defaults need a decision first: a call-depth
  limit of 128 may break real programs.
- **`and`/`or` semantics (C6).** Needs a language decision.
- **Pre-existing codegen drift.** `go run ./cmd/codegen` rewrites the
  `SPAWN_AGENT` operand row, deletes a hand-written artifact section in
  `bytecode_reference.md`, and adds `int_operand` to the orchestrator
  schema. It was left alone and is noted as P1.6.
- No changes to the production compiler, #90, the tip-lock, DOM, or the
  artifact format.

## Environment notes for reproducers
- The Codex sandbox forbids socket listeners. Whole-package
  `go test ./internal/vm` fails inside it on `httptest`, but passes on the
  box.
- The new model-call test binds 127.0.0.1:11434 only when that port is
  free. Otherwise it skips the listener-dependent part and still asserts
  the capability denial.
