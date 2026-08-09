# Direct Neural Bytecode Synthesis — Phase 1 (#49)

**Date**: 2026-07-24
**Delegate status**: agy unavailable this session (user's weekly Antigravity quota exhausted, per explicit user instruction not to use it); local Ollama not installed (`which ollama` → not found). Both rungs of the delegation ladder are down, so implementing directly per Working Protocol point 10, same precedent as #53.

## Selection

Ranked backlog scan: `bugs.md` has zero open rows (all Done, including #22 which is Done in the table but missing its detail-section note — flagged for the groom pass, not a functional bug). `improvements.md` top open (above-floor) row is #49 "Direct Neural Bytecode Synthesis" at score 1.00, ahead of #59 (0.66), #54/#52 (0.50 each). No task journals or unmerged worktree branches existed at session start; the other `claude`/`agy` processes found in this working directory (`ps aux`) were in stopped/suspended (`T`) state, not live, and `git fetch` confirmed local `HEAD` already matches `origin/main` with no unmerged remote feature branches — safe to proceed solo.

## Re-evaluation (protocol step 2)

#49 as filed has **no concrete design** ("no concrete design exists yet" per the 2026-07-24 groom note) and is scored effort-8/"weeks" — not something to attempt whole in one session. Following the exact precedent #53 set (Phase 1/2 split when a top-ranked item's full scope is too large to do solo in one sitting), this session scopes and ships a **Phase 1**: a concrete design plus a working, verified prototype that proves the item's actual core claim — bypassing text-based codegen (`go build`/`node`) entirely for a real subset of the language — rather than attempting the full "LLM emits target bytecode directly" vision, which depends on a bytecode *format* that doesn't exist yet.

## Design

Full design written to `docs/direct_execution_design.md`. Summary: a tree-walking interpreter (`interpreter.go`) that consumes the same parsed `*Node` AST `howlframe.go` already produces (same lexer/parser/`expandIncludes`/`applyPatches` front end, completely unchanged) and executes a `cli_app` script directly in the `howlframe` process — no `server.go` text is ever generated, no `go build`/`go run` subprocess is ever invoked. Wired via a new `-run` boolean flag on the existing binary.

Covered node subset (deliberately bounded, mirroring #53's "not every kind is meaningful" reasoning): `let`, `set`, `if`, `while`, `do`, `print`, `return`, the binop set (`+ - * / < > <= >= == != = and or`), `to_int`, `to_float`, `to_string`, `bytes_to_string`, `str_split`, `str_join`, `regex_match`, `append`, `map_set`, `map_delete`, `list`, `dict`, `cli_args`, `sleep`, `defun`/`call`, `read_file`, `write_file`, `mkdir`, `env`. Explicitly out of scope for Phase 1 (clear error if attempted under `-run`): `http_server`/`route`/`middleware`, `struct`/`parse_json`/`res_json`, `db_connect`/`sql_query`, `spawn`, `fetch`, `llm_generate`/`fuzzy_cast`/`assert_semantic`, `try_let`, `patch`, `with_context`, `test`, `match`, `exec`, `web_app`.

Two deliberate, documented deviations from the Go backend (not bugs — the interpreter has no equivalent parsing chokepoint to inherit them from):
1. Dynamic values (`any`-typed) instead of Go's `[]string`-only `list`/`dict` restriction.
2. `if`/`while` conditions support arbitrary recursive expressions (compound operands, `and`/`or`) since the interpreter evaluates the whole condition subtree the same way it evaluates any other expression — it does not "fix" bug #18 in the Go backend, it just isn't built the way that codegen path is.

## Verification plan

1. `go build`/`go vet` clean.
2. New fixture `tests/test_interpret_basic.howl` exercising the covered subset (arithmetic, control flow, a `defun`/`call`, string ops, list/dict).
3. Run it two ways and diff stdout: `./howlframe -run tests/test_interpret_basic.howl` vs `./howlframe tests/test_interpret_basic.howl && go run server.go` (cleaning up `server.go`/`server_test.go` after) — must match exactly for the covered subset.
4. Full `tests/*.howl` suite through the normal (unchanged) Go path to confirm zero regressions from adding the new flag/file.

## Implementation note (architecture correction mid-session)

First attempt wrote the interpreter as a separate `interpreter.go`. This broke two things: (a) without a matching `//go:build ignore` tag it got swept into every directory-wide `go build .`/`go vet ./...`/`go test ./...`, which don't have access to `howlframe.go`'s own ignore-tagged declarations (`Node`, `reportError`, `binOpKinds`, ...) — compile failure; (b) after tagging it identically, the universal documented invocation `go run howlframe.go yourfile.howl` (README, skill docs, every test script) only compiles the single file explicitly named, so `howlframe.go`'s new reference to `Interpret` stayed undefined unless every caller switched to naming both files. That's a breaking change to a workflow used everywhere, for no benefit — merged the interpreter's code directly into `howlframe.go` instead (now ~2725 lines) so the single-file invocation is untouched. `docs/direct_execution_design.md` updated to document this as an implementation note rather than leaving the design doc describing a file layout that doesn't match reality.

Also added `for` to the covered node subset while implementing (simple, valuable, was omitted from the initial design doc table — added post hoc) and confirmed `read_file`/`write_file`/`mkdir` are correctly deferred to Phase 2 alongside `try_let` (the error-tuple convention they'd need doesn't exist yet in Phase 1).

## Verification results

- `go vet howlframe.go`: caught one real issue (unreachable `return nil` after an exhaustive switch in `evalBinop` — removed) before it was clean.
- `gofmt -l howlframe.go`: clean after `gofmt -w`.
- Manual smoke test: `(cli_app (print "hi"))` produces identical `hi` output via both `go run howlframe.go -run /tmp/hitest.howl` and `go run howlframe.go /tmp/hitest.howl && go build -o /tmp/servercheck . && /tmp/servercheck`.
- Full fixture parity test (`tests/test_interpret_basic.howl`) and the full `tests/*.howl` regression suite: next step below.

## Fixture design finding: byte-identical parity is only meaningful for the Go-backend-compatible subset

First fixture draft used raw int literals in `defun` args (no `type_hint`) and a `(list 1 2 3)` of ints, on the assumption the interpreter's dynamic typing would make this trivially portable. Running it through the *unchanged* Go backend immediately failed `go build` with `mismatched types string and untyped int` (defun args default to Go `string` with no `type_hint`) and `cannot use 1 ... as string value in array or slice literal` (`list`'s codegen hardcodes `[]string{}`). This isn't a bug in either backend — it's exactly improvement #46's `type_hint`-boilerplate finding, now demonstrated concretely: the same int-add function needs three `type_hint` lines under the Go backend and zero under the interpreter. Rewrote the parity fixture to use explicit `type_hint`s and string-only list/dict elements (Go-backend-compatible), which the interpreter also runs correctly (it simply ignores `type_hint` config nodes when reading a `defun`, verified against the pre-existing `tests/test_return_compound.howl` fixture first, byte-identical). `regex_match` was dropped from the parity fixture entirely — the Go backend's `regexp.MatchString` codegen returns a bare `(bool, error)` two-value expression with no single-value wrapper (unlike `to_int`/`to_float`, which use a `func() T {...}()` wrapper specifically to be embeddable), so it cannot be used standalone in Go outside `try_let` at all; parity isn't testable for it without `try_let`, which is Phase 2 scope.

## Side finding: bugs.md #18 has a stale/incorrect detail note

While designing the parity fixture, empirically re-tested bug #18's two repros (`(if (and (> 5 1) (< 5 10)) ...)` and `(if (> (+ 2 3) 4) ...)`, plus the `while` equivalent) against current `howlframe.go` expecting them to still fail per the file's own "Groomed (2026-07-23): ... both repros ... still reproduce verbatim" note. All three now **pass** — `if`/`while` both correctly support `and`/`or` and compound operands. The ranked-table row already says "Done (2026-07-23)"; only the detail section's last groom note is stale (written before the actual fix landed later the same day, never updated afterward). Flagged for the groom pass (task queued): fix bugs.md's #18 detail section with a real "Done" note instead of leaving the contradictory stale groom note as the most recent entry. This also meant the design doc's originally-claimed "deviation #2" (if/while condition shape) was **wrong** — removed it; only two real deviations remain (list/dict element types, `for`'s list-argument expression flexibility, the latter added after discovery during implementation).

## Verification results (final)

- `tests/test_interpret_basic.howl` (rewritten, Go-backend-compatible): `./howlframe -run` output vs `./howlframe` transpile + `go build` + run output — `diff` reports **identical**, output saved and compared at `/tmp/interp_out.txt`/`/tmp/go_out.txt` during the session.
- Full existing `tests/*.howl` suite run through the unchanged transpile+build path: zero regressions. Two pre-existing, unrelated failures reproduce identically to before this change: `tests/routes.howl` (a module fragment meant for `include`, not a standalone root — expected per bug #15's Done note) and `tests/test_schema.howl` (missing `go.sum` entry for `github.com/mattn/go-sqlite3`, same class as bug #22's `uuid` issue, already noted as a known pre-existing gap in the #53 IR journal, not yet its own tracked bug — candidate for the groom pass).
- `go vet howlframe.go`: clean (after removing one real unreachable-code issue caught mid-implementation). `gofmt -l howlframe.go`: clean. `go test ./...`: all passing packages still pass (no new package added).
- Housekeeping: an untracked `telemetry.jsonl` (a runtime artifact from improvement #55's telemetry hooks, generated by the verification runs' compiled test binaries) was found in the working tree and removed; added `telemetry.jsonl` and `crash.json` (improvement #58's runtime artifact, same risk) to `.gitignore` alongside the existing `servercheck`/`*.db` entries from the 2026-07-24 groom pass note in `improvements.md` point 11.

## Next step

Close out #49 as Done (2026-07-24) — Phase 1 in `improvements.md` (same convention as #53), noting Phase 2 (bytecode serialization format + `http_server`/`try_let`/I-O coverage) and Phase 3 (an LLM emitting the bytecode/IR directly, skipping `.howl` source text entirely) as future scope, not filed as separate backlog rows yet. Then fix bugs.md #18's stale detail note and file the `test_schema.howl`/sqlite3 go.sum gap during the groom pass, commit, and archive this journal.
