# HowlFrame correctness follow-ups: HF-05, the environment policy (#182), and the deferred register

Mission of 2026-10-21. Base: HowlFrame `main` `bfc4ff8f`. Implementing agent: Claude Opus 5.5.
Status labels: **implemented**, **independently verified**, **approved**, **merged**, **deferred**. Nothing here is merged or approved.

## 1. HF-05: value-less forms in value positions (HowlFrame PR #212)

### Defect
A form that leaves nothing on the VM operand stack (`write_file`, `print`, `mkdir`, `store_put`, ...) could be used where a
value is required. `check` and every build path accepted the program. At run time the form's effect happened, and then
the VM stopped with `STACK_UNDERFLOW`. The audit's case was `(try_let (r (write_file ...)) ...)`, reproduced on `bfc4ff8f`.

### Missing contract
The checker already had the contract: `requireValue` (HFREC-009) refuses a void expression where a value is needed.
It was applied to `let` and `set` values, call arguments, operands, `list` elements and `print` arguments, but not
everywhere. The HFIR model adapter, which never runs the source checker, had no such contract at all.

### Positions found and closed

| Source of the finding | Positions | Status |
|---|---|---|
| Audit | `try_let` binding | Implemented |
| This mission's 20-position probe | `dict` value, `return` value | Implemented |
| This mission's adapter probe | HFIR adapter: any value role (`try` expression, `let` value, `print` value, `dict_entry` value, `sequence` ending in an effect) | Implemented |
| **Independent review (Codex probe matrix, round 1)** | Built-in arguments (`read_file`/`mkdir` path, `write_file` data, `sleep`, `stderr`, conversions, `encode_json`), `for` iterable, `match` subject, `if` without `else` and `match` without `default` used as values, a void call as a built-in argument, `store_put`/`store_delete` used as a value | Implemented in `692b4dfa` |
| **Independent review (Antigravity, round 2)** | A `try_let` used as a value whose success or catch body yields nothing; the same for adapter `try` nodes | Implemented in `8f45ad63` |
| **Independent review (Antigravity, round 3, confirmation)** | That `try_let` nested in a `do` tail, an `if` branch, or another `try_let`'s `do` body | Implemented in `7de10d4a` by one recursive rule |

Design, all in existing structures (no new opcode, form, capability or lowering, and no HFIR redesign):
- `internal/checker/types.go`: `requireValue` at the three missing positions. `requireValueArguments` covers an explicit
  `valueArgumentForms` set and `for` iterables, and `inferValue` covers `match` subjects. `requireValue` also refuses,
  through the recursive `yieldsNoValue`, any form with a path that yields nothing: partial `if`/`match`, store writes,
  and a `do`/`let` tail, `if` branch, `try_let` body or `match` arm that yields nothing, with exit/return paths exempt.
  It reports each node once. The root cause found across all three review rounds is that type joins turn
  void-or-value into `any`, so the check follows structure rather than the joined type.
- `internal/hfir/model_adapter.go`: `validateValueRoles` refuses a value-less node (or a `let`/`sequence` ending in one,
  an `if` without `else`, or an `if`/`try` with a value-less branch other than exit/return) wired into any non-statement role. `valueLessKinds` is pinned to the opcode table's `Pushes`
  by `TestValueLessKindsMatchOpcodeTable`.
- Not covered, by design, as a narrowness choice: `call` results whose function body yields nothing are left to the
  function's own return-type check, and forms outside `valueArgumentForms` (lambdas, sugar, structural forms) keep the runtime contract.

### Evidence
- **No effect before rejection.** `TestValuePositionVoidRejectedBeforeEffects` covers 7 shapes, including one whose
  effect is an *earlier* statement, across `check`, `build`, `-compile-bc` and `-compile-hfir-bc`, and requires an empty
  directory afterwards. The reviewer's own 31-program matrix, re-run on the corrected build, shows 25 of 25 invalid
  programs refused on all 7 paths (`check`, `build`, `bc`, `hfir`, `-run`, Go, JS) with **no effect**, and its 3 valid
  programs unchanged from `main`. Across every probe from all review rounds and this mission (59 programs), the 49 defect
  probes are refused and the 10 valid ones accepted.
- **Valid programs unaffected.** Checker verdicts were compared with `main` for **9,845 local `.howl` programs** (this repo,
  HowlBoard and other checkouts). On the final tree exactly one distinct program changed:
  `benchmarks/v2/reference/task_08_capability_restricted/howlframe/app.howl`, which *is* the defect. With `filesystem`
  granted, it wrote its secret to `leaked.txt` and then crashed. It is corrected (+4 tokens). An intermediate version
  of the correction typed `store_put` as void; the corpus diff caught that it falsely rejected HowlBoard's `server.howl`
  (`log_event`), and it was replaced. `TestModelAdapterCoversArityTable` (386 programs through the adapter) passes unchanged.
- **Mutation:** reverting the checker change fails 4 CLI and 3+ checker subtests; disabling the adapter check fails every adapter case.
- **Gates on the final tree (`7de10d4a`):** gofmt, `go build ./...`, `go vet ./...`, `go test ./...`, the write-cost, v2 and
  real-app benchmark proofs, the v2 harness unittest, and `scripts/ci_acceptance.sh` (M1 + M2 frozen and regression suites): all pass.

### Independent review status
| Round | Reviewer | Head | Outcome |
|---|---|---|---|
| 1 | Codex (OpenAI) | `dc0d0b4a` | Session **terminated by the provider's content filter**, so **no verdict**. Its surviving probe matrix found 11 more shapes, fixed in `692b4dfa` |
| 2a | Codex | `692b4dfa` | Terminated by the same filter; no evidence |
| 2b | Antigravity (Google) | `692b4dfa` | **REJECT**: 1 class (a `try_let` body in a value position); everything else confirmed, including the opcode table and that no test was weakened. Fixed in `8f45ad63` |
| 3 | Antigravity (bounded confirmation) | `8f45ad63` | **REJECT**: nested variants. Fixed in `7de10d4a` by the recursive rule |
| — | Implementer | `7de10d4a` | All reviewers' probes re-run as above |
| 4 | Antigravity (bounded confirmation, 2026-10-22, at the Owner's request) | `7de10d4a` | **ACCEPT**: every round-3 variant and 20 new near variants refused on all four compile paths with no file written; valid diverging, statement-position and `log_event` code accepted and run; corpus 0 differences; gofmt, vet and tests clean (`evidence/reviews/hf05_antigravity_round4.md`) |

Reviews found a new variant of one root cause in each round. The final rule addresses that root cause structurally,
and round 4 confirmed it independently. Merged 2026-10-22 as `6912b0bb`.

## 2. HF-11 / #182: `process:env` forwarding policy (step 1 implemented 2026-10-22; step 2 open)

**Update 2026-10-22:** the Owner approved step 1 (decision H). It is implemented as the class *execution-steering* below,
with the prefix, suffix and name tables listed in `change_log.md`. Step 2 (an allowlist) is not implemented.

### Current behavior (verified on `bfc4ff8f`)
Exec children start with an **empty** environment. Each `process:env=NAME` grant forwards one host variable, unless the
name is in a single-sourced refusal table (`capability.CodeLoadingEnvNames` and `CodeLoadingEnvPrefixes`: loader, shell,
interpreter-path, Go/Java, libc lookup, trust-store names, plus `PATH` and `HOME`). Values come from the host, never from
the program, and each name needs an explicit operator grant. The audit's gaps are still forwarded when granted:
`OPENSSL_CONF`, `OPENSSL_MODULES`, `GLIBC_TUNABLES`, `MALLOC_CHECK_`, `*_PROXY` (both cases), `NODE_USE_ENV_PROXY`,
`GEM_PATH`/`GEM_HOME`, `RUBYGEMS_GEMDEPS`, `PHPRC`, `ERL_LIBS`, `TMPDIR`, `LESSOPEN`, `PAGER`, `EDITOR`.

### Consumers (measured)
Every `process:env=` use across HowlFrame, HowlBoard, HowlPlane, HowlWriter, Howl, HowlCreate, HowlDream, HowlRelay and
HowlChangeOps is in **HowlFrame's own tests**, plus **TaskFlow's frozen acceptance test** `T-R11-env-isolation`
(`process:env=HF_TEST_PASS`). There is no production consumer in the ecosystem. The parity tests require
`LANG`, `TZ`, `HOWL_TOKEN`, `LOGNAME`, `PATHNAME`, `HOMEDIR`, `GITHUB_SHA` and `XDG_RUNTIME_DIR` to keep passing.

### Proposed policy: three classes

| Class | Rule | Examples |
|---|---|---|
| **Execution-steering** | Always refused, whatever is granted | Today's table, plus prefix families `OPENSSL_`, `GLIBC_`, `MALLOC_`, `GEM_`, `BUNDLE_`, `RUBYGEMS_`, `PHP`, `ERL_`, `PYTHON`, `NODE_`, `PERL`, `RUBY`, `JAVA_`, `JDK_`, `CARGO_`, `NPM_CONFIG_`, `YARN_`, `PIP_`, `SSL_`, `CURL_`; suffix `_PROXY` (case-insensitive); names `PAGER`, `EDITOR`, `VISUAL`, `LESSOPEN`, `LESSCLOSE`, `BROWSER`, `SHELL`, `TMPDIR`, `TMP`, `TEMP`, `SSH_ASKPASS`, `SUDO_ASKPASS`, `RES_OPTIONS`, `LOCALDOMAIN`, `TERMINFO`, `MANPATH`, and Go toolchain names (`GOPROXY`, `GOPRIVATE`, `GONOSUMDB`, `GOINSECURE`, ...) as exact names: a bare `GO` prefix would catch application names such as `GOAL_ID` |
| **Inert data** | Forwarded with `process:env=NAME` | `LANG`, `LANGUAGE`, `LC_*`, `TZ`, `TERM`, `COLUMNS`, `LINES`, `NO_COLOR`, `LOGNAME`, `USER`, `CI`, `SOURCE_DATE_EPOCH` |
| **Application-defined** | Step 1: forwarded with `process:env=NAME` (as today). Step 2 (optional): forwarded only with an explicit trusted-mode grant, for example `process:env-app=NAME` | `HF_TEST_PASS`, `HOWL_TOKEN`, `GITHUB_SHA` |

**Measured impact of step 1** (a prototype classifier, not repository code): all 19 audit and #182 gap names move to
*refused*, and all 14 known consumer names keep their current behavior. In this host's environment (111 names, counted by
class only), 13 are execution-steering, 6 of them newly refused if anyone granted them.

**Step 2** is a true allowlist. It **breaks** TaskFlow's frozen `T-R11-env-isolation` (an application name) and the parity
`EnvAllowed` list, so it needs a recorded requirements revision for M1 TaskFlow. That is an Owner decision, not an agent decision.

**Migration:** step 1 turns a previously parsed `process:env=OPENSSL_CONF` (and similar) into a parse-time error on the
interpreter and `-run-bc`, and into a skipped grant in generated Go/JS, matching how the #166/#172/#177 additions were
rolled out. The change log must list every added family.

**Implementation, if approved:** extend `CodeLoadingEnvNames` and `CodeLoadingEnvPrefixes` (plus a suffix rule),
render the same tables into generated Go and JS, and add positive and negative cases to `internal/nettest/env.go`
(`EnvRefused`, `EnvAllowed`). Owner decision H.

## 3. Deferred register (not in scope for this mission)

| ID | Finding | Status | Note |
|---|---|---|---|
| HF-12 | Runner-limit stops print raw runtime JSON (`LIMIT_EXCEEDED`), conflicting with frozen hygiene rules; the m12 XFAIL | Deferred | Also seen here: HowlBoard's import can still hit a raw memory-limit error if an operator raises `MAX` more than 10× |
| HF-13 | Stores and `write_file` create files with mode 0644 | Deferred | TicketDesk state holds token digests |
| HF-14 | `--max-memory-bytes` is a cumulative charge, not a live bound | Deferred | Measured here: the new HowlBoard importer exceeds a 256 MiB charge on a 4.1 MB slice, while a 3.0 MB slice passes at 234 MB RSS. The charge, not real memory, limits slice size |
| HF-20 | Servers listen on all interfaces by default | Deferred | `--listen-host` exists |
| — | Hostile-code process isolation (supervisor kill) | Deferred | Owner decision E; needs a new issue |
| — | HowlBoard import write phase: each mission is a separate `store_put` (whole-file rewrite). It is O(n²) in missions, and a crash mid-phase leaves missions updated with the old timeline | Deferred (pre-existing; identical in the old importer) | 40-kill storm: 0 torn files, 15/40 partial (new), 27/40 partial (old) |
| — | EvidenceDesk `verify_workflows.py` cannot run against current upstream HowlBoard (the R5 adapter is a compile error there) | Deferred to Owner decision D | Passes only against the stale `cd2f2fd` checkout |
| — | `docs/installation.md` says Go 1.21+ | Library update candidate | `go.mod` requires 1.25 (Owner decision A) |
| — | `str_len` documentation says bytes; behavior counts characters | Library update candidate | Found by the HowlBoard reviewer (`internal/ast/desugar_corelib.go`) |
| #147, #194 | Test flakes (fixed port; SIGSTOP self-test) | Open, unchanged | Not touched |
