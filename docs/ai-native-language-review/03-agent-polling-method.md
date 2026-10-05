# 03: Agent polling method

## Goal

Get at least four independent views of HowlFrame's AI-native direction.
Each reviewer read the real repository at `d496d97`, answered the same 14
questions, and added a section for its specialist role (A to F). No
reviewer saw another reviewer's answer. Skepticism was explicitly
rewarded in every prompt.

## The brief, as reconstructed

The verbatim user brief (its "listed questions" and "roles A to F") was not
available to the executor, so this review rebuilt both from the mission
statement. Both are recorded here so they can be audited.

- **Common prompt:** `/tmp/hf-ainative-agents/common.md`, copied into
  `raw/common-prompt.md`. It contains the thesis, the standing
  constraints, a mandatory reading list (code first), the citation rule
  (`path:line`, each claim tagged VERIFIED or INFERRED), the 14 questions,
  and the output format.
- **The 14 questions:**
  1. What HowlFrame is today.
  2. What it is becoming.
  3. Whether the thesis is sound.
  4. Who would use it.
  5. Its trusted computing base (TCB).
  6. Whether the `.howl` syntax matters, or a model-facing contract is better.
  7. How it compares with prior art.
  8. Which features conflict with the thesis.
  9. Its top 5 risks.
  10. What the authority model should become.
  11. What "verified" means.
  12. A P0 to P4 roadmap.
  13. A first falsifiable experiment.
  14. Advice for the next two weeks.
- **Roles:**

  | Role | Focus |
  | --- | --- |
  | A | Compiler and runtime architect |
  | B | Security and capability red team (build and run probes) |
  | C | AI/LLM program-synthesis researcher |
  | D | Skeptic and devil's advocate |
  | E | Competitive and prior-art analyst |
  | F | Dogfood, developer experience, and adoption engineer |

## Tools and models

| Role | Engine | Model | Mode | Repo access |
| --- | --- | --- | --- | --- |
| A | Codex CLI `codex exec -s workspace-write` | Codex default | Shell allowed, read-only by instruction | Isolated clone `repo-A` at `d496d97` |
| B | Codex CLI (same flags) | Codex default | Ran probes against a local 127.0.0.1 listener | `repo-B` |
| C | AGY `agy -p --mode plan` | `gemini-3.1-pro-high` | No shell (see below) | `repo-C` plus `REVIEW_CONTEXT_PACK.md` |
| D | Codex CLI | Codex default | Shell allowed | `repo-D` |
| E | AGY `agy -p --mode plan` | `gemini-3.8-flash-high` | No shell | `repo-E` plus context pack |
| F | Codex CLI | Codex default | Shell allowed, ran the README and the action executor | `repo-F` |

No run used `--dangerously-skip-permissions` or any equivalent flag.

## What failed, and how this review worked around it

- **Claude CLI** is weekly-limited until about Oct 7, 2 pm ET. It was not
  used.
- **AGY with Claude, Opus, Sonnet, or GPT-OSS models:** the first attempts
  exited early or hit HTTP 429 quota errors. The quota resets about 4 h 50 m
  after 19:3x ET.
- **AGY headless mode denies `run_command`.** Claude-family models then
  stopped without answering. Two changes followed:
  1. A box-local `~/.gemini/antigravity-cli/settings.json` was added with
     read-only `command(...)` allow rules. It sits outside the repo and
     affects only this box.
  2. Each AGY clone got a `REVIEW_CONTEXT_PACK.md` (~300 KB). The pack
     holds the mandatory reading files, unmodified at `d496d97`, plus
     `git log --oneline -40`. AGY prompts told the agent not to use the
     shell.
- **Consequence:** C and E could not run code. Their line numbers often
  refer to positions in the context pack, not in the real file. E cites
  `howlframe.go:1362`, but the file has 941 lines. Both were graded down
  for this (see 04).
- **Codex sandbox:** the sandbox forbids opening sockets, so the Codex
  reviewers' whole-package `go test ./internal/vm` runs failed on
  `httptest` listeners (`ast_interp_cap_test.go:306`). This is an
  environment limit, not a repo failure. The same suite passes on the box
  (38/38 packages; see MISSION_JOURNAL).
- **E's output** came back malformed. The header is duplicated and the
  file stops partway through question 8. It is kept in `raw/` as
  returned.

## Independence controls

- Each reviewer had its own clone and scratch directory
  (`/tmp/hf-ainative-scratch-<ROLE>`) and its own `GOCACHE`.
- Prompts contained no executor findings. In particular they did not
  mention the model-call capability hole that this review had already
  reproduced. Prompts named areas to audit ("model/LLM calls") but gave
  no conclusions.
- All reviewers ran concurrently. None could read another's output file.
- Grading against the code (04 and 06) happened only after every answer
  was in.

## Limits

- Four Codex runs share one model family. A, B, D, and F agree partly
  because they come from the same engine. The two Gemini runs add
  diversity but were the weakest on evidence.
- No Claude-family perspective took part because of the quota and
  headless limits.
- No human reviewer took part.
